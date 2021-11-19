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
	"io"
	"path/filepath"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

func newDependencyListCmd(out io.Writer) *cobra.Command {
	var outfmt output.Format
	client := action.NewDependency()
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

			dependencies, err := action.ListDependencies(chartpath, out)

			if err != nil {
				return err
			}

			if dependencies != nil {
				if outfmt == output.Table {
					action.PrintMissing(chartpath, out, dependencies)
				}

				var deplist = make([]dependencyElement, 0, len(dependencies))

				for _, d := range dependencies {
					deplist = append(deplist, dependencyElement{Name: d.Name, Version: d.Version, Repository: d.Repository, Status: d.Status})
				}

				return outfmt.Write(out, &dependencyListWriter{deps: deplist, columnWidth: client.ColumnWidth})
			}

			return nil
		},
	}

	f := cmd.Flags()

	f.UintVar(&client.ColumnWidth, "max-col-width", 80, "maximum column width for output table")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

type dependencyElement struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Status     string `json:"status"`
}

type dependencyListWriter struct {
	deps        []dependencyElement
	columnWidth uint
}

func (d *dependencyListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.MaxColWidth = d.columnWidth
	table.AddRow("NAME", "VERSION", "REPOSITORY", "STATUS")
	for _, row := range d.deps {
		table.AddRow(row.Name, row.Version, row.Repository, row.Status)
	}
	return output.EncodeTable(out, table)
}

func (d *dependencyListWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, d.deps)
}

func (d *dependencyListWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, d.deps)
}
