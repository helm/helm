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
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
)

const dependencyUpDesc = `
Update the on-disk dependencies to mirror Chart.yaml.

This command verifies that the required charts, as expressed in 'Chart.yaml',
are present in 'charts/' and are at an acceptable version. It will pull down
the latest charts that satisfy the dependencies, and clean up old dependencies.

On successful update, this will generate a lock file that can be used to
rebuild the dependencies to an exact version.

Dependencies are not required to be represented in 'Chart.yaml'. For that
reason, an update command will not remove charts unless they are (a) present
in the Chart.yaml file, but (b) at the wrong version.
`

// newDependencyUpdateCmd creates a new dependency update command.
func newDependencyUpdateCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewDependency()

	cmd := &cobra.Command{
		Use:     "update CHART",
		Aliases: []string{"up"},
		Short:   "update charts/ based on the contents of Chart.yaml",
		Long:    dependencyUpDesc,
		Args:    require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartpath := "."
			if len(args) > 0 {
				chartpath = filepath.Clean(args[0])
			}
			man := &downloader.Manager{
				Out:              out,
				ChartPath:        chartpath,
				Keyring:          client.Keyring,
				SkipUpdate:       client.SkipRefresh,
				Getters:          getter.All(settings),
				RegistryClient:   cfg.RegistryClient,
				RepositoryConfig: settings.RepositoryConfig,
				RepositoryCache:  settings.RepositoryCache,
				Debug:            settings.Debug,
			}
			if client.Verify {
				man.Verify = downloader.VerifyAlways
			}
			return man.Update()
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.Verify, "verify", false, "verify the packages against signatures")
	f.StringVar(&client.Keyring, "keyring", defaultKeyring(), "keyring containing public keys")
	f.BoolVar(&client.SkipRefresh, "skip-refresh", false, "do not refresh the local repository cache")

	return cmd
}
