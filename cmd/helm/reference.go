/*
Copyright The Helm Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/gosuri/uitable"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
)

type refListOptions struct {
	chartpath string
}

func (o *refListOptions) run(out io.Writer, isLib bool) error {
	c, err := loader.Load(o.chartpath)
	if err != nil {
		return err
	}

	var reqs []*chart.Dependency
	var dirName string
	if isLib {
		reqs = c.Metadata.Libraries
		dirName = "library"
	} else {
		reqs = c.Metadata.Dependencies
		dirName = "charts"
	}

	if reqs == nil {
		if isLib {
			fmt.Fprintf(out, "WARNING: no libraries at %s\n", filepath.Join(o.chartpath, dirName))
		} else {
			fmt.Fprintf(out, "WARNING: no dependencies at %s\n", filepath.Join(o.chartpath, dirName))
		}
		return nil
	}

	o.printDependencies(out, reqs, dirName)
	fmt.Fprintln(out)
	o.printMissing(out, reqs, dirName)
	return nil
}

func (o *refListOptions) dependencyStatus(dep *chart.Dependency, dirName string) string {
	filename := fmt.Sprintf("%s-%s.tgz", dep.Name, "*")
	archives, err := filepath.Glob(filepath.Join(o.chartpath, dirName, filename))
	if err != nil {
		return "bad pattern"
	} else if len(archives) > 1 {
		return "too many matches"
	} else if len(archives) == 1 {
		archive := archives[0]
		if _, err := os.Stat(archive); err == nil {
			c, err := loader.Load(archive)
			if err != nil {
				return "corrupt"
			}
			if c.Name() != dep.Name {
				return "misnamed"
			}

			if c.Metadata.Version != dep.Version {
				constraint, err := semver.NewConstraint(dep.Version)
				if err != nil {
					return "invalid version"
				}

				v, err := semver.NewVersion(c.Metadata.Version)
				if err != nil {
					return "invalid version"
				}

				if constraint.Check(v) {
					return "ok"
				}
				return "wrong version"
			}
			return "ok"
		}
	}

	folder := filepath.Join(o.chartpath, dirName, dep.Name)
	if fi, err := os.Stat(folder); err != nil {
		return "missing"
	} else if !fi.IsDir() {
		return "mispackaged"
	}

	c, err := loader.Load(folder)
	if err != nil {
		return "corrupt"
	}

	if c.Name() != dep.Name {
		return "misnamed"
	}

	if c.Metadata.Version != dep.Version {
		constraint, err := semver.NewConstraint(dep.Version)
		if err != nil {
			return "invalid version"
		}

		v, err := semver.NewVersion(c.Metadata.Version)
		if err != nil {
			return "invalid version"
		}

		if constraint.Check(v) {
			return "unpacked"
		}
		return "wrong version"
	}

	return "unpacked"
}

// printDependencies prints all of the dependencies/libraries in the yaml file.
func (o *refListOptions) printDependencies(out io.Writer, reqs []*chart.Dependency, dirName string) {
	table := uitable.New()
	table.MaxColWidth = 80
	table.AddRow("NAME", "VERSION", "REPOSITORY", "STATUS")
	for _, row := range reqs {
		table.AddRow(row.Name, row.Version, row.Repository, o.dependencyStatus(row, dirName))
	}
	fmt.Fprintln(out, table)
}

// printMissing prints warnings about charts that are present on disk, but are
// not in Charts.yaml.
func (o *refListOptions) printMissing(out io.Writer, reqs []*chart.Dependency, dirName string) {
	folder := filepath.Join(o.chartpath, dirName+"/*")
	files, err := filepath.Glob(folder)
	if err != nil {
		fmt.Fprintln(out, err)
		return
	}

	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			fmt.Fprintf(out, "Warning: %s\n", err)
		}
		// Skip anything that is not a directory and not a tgz file.
		if !fi.IsDir() && filepath.Ext(f) != ".tgz" {
			continue
		}
		c, err := loader.Load(f)
		if err != nil {
			fmt.Fprintf(out, "WARNING: %q is not a chart.\n", f)
			continue
		}
		found := false
		for _, d := range reqs {
			if d.Name == c.Name() {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(out, "WARNING: %q is not in Chart.yaml.\n", f)
		}
	}

}
