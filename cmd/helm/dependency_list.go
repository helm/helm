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

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli/output"
)

func newDependencyListCmd(out io.Writer) *cobra.Command {
	var outfmt output.Format
	cmd := &cobra.Command{
		Use:     "list CHART",
		Aliases: []string{"ls"},
		Short:   "list the dependencies for the given chart",
		Long:    dependencyListDesc,
		Args:    require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartpath := "."
			if len(args) > 0 {
				chartpath = filepath.Clean(args[0])
			}
			return outfmt.Write(out, &dependencyListWriter{chartpath: chartpath})
		},
	}

	bindOutputFlag(cmd, &outfmt)

	return cmd
}

// dependencyListElement is a single element in json/yaml array of dependencies
type dependencyListElement struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Status     string `json:"status"`
}

// dependencyListWriter implements 'output.Format' interface
type dependencyListWriter struct {
	chartpath string
}

func (w *dependencyListWriter) WriteTable(out io.Writer) error {
	c, err := loader.Load(w.chartpath)
	if err != nil {
		return err
	}

	if c.Metadata.Dependencies == nil {
		fmt.Fprintf(out, "WARNING: no dependencies at %s\n", filepath.Join(w.chartpath, "charts"))
		return nil
	}

	w.printDependenciesTable(w.chartpath, out, c)
	fmt.Fprintln(out)
	w.printMissing(w.chartpath, out, c.Metadata.Dependencies)
	return nil
}

func (w *dependencyListWriter) WriteJSON(out io.Writer) error {
	return w.encodeByFormat(out, output.JSON)
}

func (w *dependencyListWriter) WriteYAML(out io.Writer) error {
	return w.encodeByFormat(out, output.YAML)
}

func (w *dependencyListWriter) encodeByFormat(out io.Writer, format output.Format) error {
	c, err := loader.Load(w.chartpath)
	if err != nil {
		return err
	}

	// Initialize the array so no results returns an empty array instead of null
	elements := make([]dependencyListElement, 0, len(c.Metadata.Dependencies))

	for _, d := range c.Metadata.Dependencies {
		elements = append(elements, dependencyListElement{Name: d.Name, Repository: d.Repository, Status: action.DependencyStatus(w.chartpath, d, c), Version: d.Version})
	}

	switch format {
	case output.JSON:
		return output.EncodeJSON(out, elements)
	case output.YAML:
		return output.EncodeYAML(out, elements)
	}

	return nil
}

// printDependenciesTable prints all of the dependencies for Table output format.
func (w *dependencyListWriter) printDependenciesTable(chartpath string, out io.Writer, c *chart.Chart) {
	table := uitable.New()
	table.MaxColWidth = 80
	table.AddRow("NAME", "VERSION", "REPOSITORY", "STATUS")
	for _, row := range c.Metadata.Dependencies {
		table.AddRow(row.Name, row.Version, row.Repository, action.DependencyStatus(chartpath, row, c))
	}
	fmt.Fprintln(out, table)
}

// printMissing prints warnings about charts that are present on disk, but are
// not in Charts.yaml for Table output format.
func (w *dependencyListWriter) printMissing(chartpath string, out io.Writer, reqs []*chart.Dependency) {
	folder := filepath.Join(chartpath, "charts/*")
	files, err := filepath.Glob(folder)
	if err != nil {
		fmt.Fprintln(out, err)
		return
	}

	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			fmt.Fprintf(out, "WARNING: %s\n", err)
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
