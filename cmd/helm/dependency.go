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

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
)

const dependencyDesc = `
Manage the dependencies of a chart.

Helm charts store their dependencies in 'charts/'. For chart developers, it is
often easier to manage dependencies in 'Chart.yaml' which declares all
dependencies.

The dependency commands operate on that file, making it easy to synchronize
between the desired dependencies and the actual dependencies stored in the
'charts/' directory.

For example, this Chart.yaml declares two dependencies:

    # Chart.yaml
    dependencies:
    - name: nginx
      version: "1.2.3"
      repository: "https://example.com/charts"
    - name: memcached
      version: "3.2.1"
      repository: "https://another.example.com/charts"


The 'name' should be the name of a chart, where that name must match the name
in that chart's 'Chart.yaml' file.

The 'version' field should contain a semantic version or version range.

The 'repository' URL should point to a Chart Repository. Helm expects that by
appending '/index.yaml' to the URL, it should be able to retrieve the chart
repository's index. Note: 'repository' can be an alias. The alias must start
with 'alias:' or '@'.

Starting from 2.2.0, repository can be defined as the path to the directory of
the dependency charts stored locally. The path should start with a prefix of
"file://". For example,

    # Chart.yaml
    dependencies:
    - name: nginx
      version: "1.2.3"
      repository: "file://../dependency_chart/nginx"

If the dependency chart is retrieved locally, it is not required to have the
repository added to helm by "helm add repo". Version matching is also supported
for this case.

A repository can be defined as a git URL. The path must start with a prefix of
"git://" followed by a valid git repository URL.

    # Chart.yaml
    dependencies:
    - name: helm-chart
      version: "main"
      repository: "git://https://github.com/helm/helm-chart.git"

The 'repository' can be the https or ssh URL that you would use to clone a git
repo or add as a git remote, prefixed with 'git:'.
For example 'git://git@github.com:helm/helm-chart.git' or
'git://https://github.com/helm/helm-chart.git'

When using a 'git://' repository, the 'version' must be a valid  semantic tag or branch
name for the git repo. For example 'master'.

Limitations when working with git repositories:
* Helm will use the 'git' executable on your system to retrieve information
about the repo. The 'git' command must be properly configured and available
on the PATH.
* When specifying a private repo, if git tries to query the user for
username/passowrd for an HTTPS url, or for a certificate password for an SSH
url, it may cause Helm to hang. Input is not forwarded to the child git
process, so it will not be able to receive user input. For private repos
it is recommended to use an SSH git url, and have your git client configured
with an SSH cert that does not require a password.
* The helm chart and 'Chart.yaml' must be in the root of the git repo.
The chart cannot be loaded from a subdirectory.
`

const dependencyListDesc = `
List all of the dependencies declared in a chart.

This can take chart archives and chart directories as input. It will not alter
the contents of a chart.

This will produce an error if the chart cannot be loaded.
`

func newDependencyCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dependency update|build|list",
		Aliases: []string{"dep", "dependencies"},
		Short:   "manage a chart's dependencies",
		Long:    dependencyDesc,
		Args:    require.NoArgs,
	}

	cmd.AddCommand(newDependencyListCmd(out))
	cmd.AddCommand(newDependencyUpdateCmd(cfg, out))
	cmd.AddCommand(newDependencyBuildCmd(cfg, out))

	return cmd
}

func newDependencyListCmd(out io.Writer) *cobra.Command {
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
			return client.List(chartpath, out)
		},
	}

	f := cmd.Flags()

	f.UintVar(&client.ColumnWidth, "max-col-width", 80, "maximum column width for output table")
	return cmd
}
