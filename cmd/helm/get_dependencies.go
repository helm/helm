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

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli/output"
)

var getDependenciesHelp = `
This command list all dependencies for a given release.
`

type dependencyElement struct {
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Repository string `json:"repository"`
}

type dependencyListWriter struct {
	dependencies []dependencyElement
}

func newGetDependenciesCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var outfmt output.Format
	client := action.NewGet(cfg)

	cmd := &cobra.Command{
		Use:   "dependencies RELEASE_NAME",
		Short: "download the dependencies for a named release",
		Long:  getDependenciesHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := client.Run(args[0])

			if err != nil {
				return err
			}

			return outfmt.Write(out, newDependenciesListWriter(res.Chart.Metadata.Dependencies))
		},
	}

	// Function providing dynamic auto-completion
	completion.RegisterValidArgsFunc(cmd, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) != 0 {
			return nil, completion.BashCompDirectiveNoFileComp
		}

		return compListReleases(toComplete, cfg)
	})

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "get the named release with revision")
	flag := f.Lookup("revision")

	completion.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) == 1 {
			return compListRevisions(cfg, args[0])
		}

		return nil, completion.BashCompDirectiveNoFileComp
	})

	bindOutputFlag(cmd, &outfmt)

	return cmd
}

func newDependenciesListWriter(dependencies []*chart.Dependency) *dependencyListWriter {
	// Initialize the array so no results returns an empty array instead of null
	elements := make([]dependencyElement, 0, len(dependencies))

	for _, d := range dependencies {
		element := dependencyElement{
			Name:       d.Name,
			Version:    d.Version,
			Repository: d.Repository,
		}

		elements = append(elements, element)
	}

	return &dependencyListWriter{elements}
}

func (d dependencyListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()

	table.AddRow("NAME", "VERSION", "REPOSITORY")

	for _, r := range d.dependencies {
		table.AddRow(r.Name, r.Version, r.Repository)
	}

	return output.EncodeTable(out, table)
}

func (d dependencyListWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, d.dependencies)
}

func (d dependencyListWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, d.dependencies)
}
