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

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
)

const libraryDesc = `
Manage the libraries of a chart.

Helm charts store their libraries in 'library/'. For chart developers, it is
often easier to manage libraries in 'Chart.yaml' which declares all
libraries.

The library commands operate on that file, making it easy to synchronize
between the desired libraries and the actual libraries stored in the
'library/' directory.

For example, this Chart.yaml declares one library:

    # Chart.yaml
    libraries:
    - name: common
      version: "^2.1.0"
      repository: http://another.example.com/charts


The 'name' should be the name of a chart, where that name must match the name
in that chart's 'Chart.yaml' file.

The 'version' field should contain a semantic version or version range.

The 'repository' URL should point to a Chart Repository. Helm expects that by
appending '/index.yaml' to the URL, it should be able to retrieve the chart
repository's index. Note: 'repository' can be an alias. The alias must start
with 'alias:' or '@'.

Starting from 2.2.0, repository can be defined as the path to the directory of
the library charts stored locally. The path should start with a prefix of
"file://". For example,

    # Chart.yaml
    libraries:
    - name: common
      version: "^2.1.0"
      repository: "file://../library_chart/common"

If the library chart is retrieved locally, it is not required to have the
repository added to helm by "helm add repo". Version matching is also supported
for this case.
`

const libraryListDesc = `
List all of the libraries declared in a chart.

This can take chart archives and chart directories as input. It will not alter
the contents of a chart.

This will produce an error if the chart cannot be loaded.
`

func newLibraryCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "library update|build|list",
		Aliases: []string{"lib", "libraries"},
		Short:   "manage a chart's libraries",
		Long:    libraryDesc,
		Args:    require.NoArgs,
	}

	cmd.AddCommand(newLibraryListCmd(out))
	cmd.AddCommand(newLibraryUpdateCmd(out))
	cmd.AddCommand(newLibraryBuildCmd(out))

	return cmd
}

func newLibraryListCmd(out io.Writer) *cobra.Command {
	o := &refListOptions{
		chartpath: ".",
	}

	cmd := &cobra.Command{
		Use:     "list CHART",
		Aliases: []string{"ls"},
		Short:   "list the libraries for the given chart",
		Long:    libraryListDesc,
		Args:    require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.chartpath = filepath.Clean(args[0])
			}
			return o.run(out, true)
		},
	}
	return cmd
}
