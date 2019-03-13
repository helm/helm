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

package loader

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
)

// ChartLoader loads a chart.
type ChartLoader interface {
	Load() (*chart.Chart, error)
}

// Loader returns a new ChartLoader appropriate for the given chart name
func Loader(name string) (ChartLoader, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return DirLoader(name), nil
	}
	return FileLoader(name), nil

}

// Load takes a string name, tries to resolve it to a file or directory, and then loads it.
//
// This is the preferred way to load a chart. It will discover the chart encoding
// and hand off to the appropriate chart reader.
//
// If a .helmignore file is present, the directory loader will skip loading any files
// matching it. But .helmignore is not evaluated when reading out of an archive.
func Load(name string) (*chart.Chart, error) {
	l, err := Loader(name)
	if err != nil {
		return nil, err
	}
	return l.Load()
}

// BufferedFile represents an archive file buffered for later processing.
type BufferedFile struct {
	Name string
	Data []byte
}

// LoadFiles loads from in-memory files.
func LoadFiles(files []*BufferedFile) (*chart.Chart, error) {
	c := new(chart.Chart)
	subcharts := make(map[string][]*BufferedFile)

	for _, f := range files {
		switch {
		case f.Name == "Chart.yaml":
			c.Metadata = new(chart.Metadata)
			if err := yaml.Unmarshal(f.Data, c.Metadata); err != nil {
				return c, errors.Wrap(err, "cannot load Chart.yaml")
			}
		case f.Name == "Chart.lock":
			c.Lock = new(chart.Lock)
			if err := yaml.Unmarshal(f.Data, &c.Lock); err != nil {
				return c, errors.Wrap(err, "cannot load Chart.lock")
			}
		case f.Name == "values.yaml":
			c.Values = make(map[string]interface{})
			if err := yaml.Unmarshal(f.Data, &c.Values); err != nil {
				return c, errors.Wrap(err, "cannot load values.yaml")
			}
			c.RawValues = f.Data
		case f.Name == "values.schema.json":
			c.Schema = f.Data
		case strings.HasPrefix(f.Name, "templates/"):
			c.Templates = append(c.Templates, &chart.File{Name: f.Name, Data: f.Data})
		case strings.HasPrefix(f.Name, "charts/"):
			if filepath.Ext(f.Name) == ".prov" {
				c.Files = append(c.Files, &chart.File{Name: f.Name, Data: f.Data})
				continue
			}

			fname := strings.TrimPrefix(f.Name, "charts/")
			cname := strings.SplitN(fname, "/", 2)[0]
			subcharts[cname] = append(subcharts[cname], &BufferedFile{Name: fname, Data: f.Data})
		default:
			c.Files = append(c.Files, &chart.File{Name: f.Name, Data: f.Data})
		}
	}

	if err := c.Validate(); err != nil {
		return c, err
	}

	for n, files := range subcharts {
		var sc *chart.Chart
		var err error
		switch {
		case strings.IndexAny(n, "_.") == 0:
			continue
		case filepath.Ext(n) == ".tgz":
			file := files[0]
			if file.Name != n {
				return c, errors.Errorf("error unpacking tar in %s: expected %s, got %s", c.Name(), n, file.Name)
			}
			// Untar the chart and add to c.Dependencies
			var isLibChart bool
			isLibChart, err = IsArchiveLibraryChart(bytes.NewBuffer(file.Data))
			if err == nil {
				sc, err = LoadArchive(bytes.NewBuffer(file.Data), isLibChart)
			}
		default:
			// Need to ascertain if the subchart is a library chart as will parse files
			// differently from a standard application chart
			var isLibChart = false
			for _, f := range files {
				parts := strings.SplitN(f.Name, "/", 2)
				if len(parts) < 2 {
					continue
				}
				name := parts[1]
				if name == "Chart.yaml" {
					isLibChart, err = IsLibraryChart(f.Data)
					if err != nil {
						return c, errors.Wrapf(err, "cannot load Chart.yaml for %s in %s", n, c.Name())
					}
					break
				}
			}

			// We have to trim the prefix off of every file, and ignore any file
			// that is in charts/, but isn't actually a chart.
			buff := make([]*BufferedFile, 0, len(files))
			for _, f := range files {
				parts := strings.SplitN(f.Name, "/", 2)
				if len(parts) < 2 {
					continue
				}
				f.Name = parts[1]

				// If the subchart is a library chart then will only want
				// to render library and value files, and not any included template files
				if IsFileValid(f.Name, isLibChart) {
					buff = append(buff, f)
				}
			}
			sc, err = LoadFiles(buff)
		}

		if err != nil {
			return c, errors.Wrapf(err, "error unpacking %s in %s", n, c.Name())
		}

		c.AddDependency(sc)
	}

	return c, nil
}

// IsLibraryChart return true if it is a library chart
func IsLibraryChart(data []byte) (bool, error) {
	var isLibChart = false
	metadata := new(chart.Metadata)
	if err := yaml.Unmarshal(data, metadata); err != nil {
		return false, errors.Wrapf(err, "cannot load data")
	}
	if strings.EqualFold(metadata.Type, "library") {
		isLibChart = true
	}
	return isLibChart, nil
}

// IsFileValid return true if this file is valid for library or
//  application chart
func IsFileValid(filename string, isLibChart bool) bool {
	if isLibChart {
		if strings.HasPrefix(filepath.Base(filename), "_") || !strings.HasPrefix(filepath.Dir(filename), "template") {
			return true
		}
		return false
	}
	return true
}
