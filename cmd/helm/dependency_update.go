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
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/helmpath"
)

const dependencyUpDesc = `
Update the on-disk dependencies to mirror Chart.yaml.

This command verifies that the required charts, as expressed in 'Chart.yaml',
are present in 'charts/' and are at an acceptable version. It will pull down
the latest charts that satisfy the dependencies, and clean up old dependencies.

On successful update, this will generate a lock file that can be used to
rebuild the requirements to an exact version.

Dependencies are not required to be represented in 'Chart.yaml'. For that
reason, an update command will not remove charts unless they are (a) present
in the Chart.yaml file, but (b) at the wrong version.
`

// dependencyUpdateOptions describes a 'helm dependency update'
type dependencyUpdateOptions struct {
	keyring     string // --keyring
	skipRefresh bool   // --skip-refresh
	verify      bool   // --verify

	// args
	chartpath string

	helmhome helmpath.Home
}

// newDependencyUpdateCmd creates a new dependency update command.
func newDependencyUpdateCmd(out io.Writer) *cobra.Command {
	o := &dependencyUpdateOptions{
		chartpath: ".",
	}

	cmd := &cobra.Command{
		Use:     "update CHART",
		Aliases: []string{"up"},
		Short:   "update charts/ based on the contents of Chart.yaml",
		Long:    dependencyUpDesc,
		Args:    require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.chartpath = filepath.Clean(args[0])
			}
			o.helmhome = settings.Home
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.verify, "verify", false, "verify the packages against signatures")
	f.StringVar(&o.keyring, "keyring", defaultKeyring(), "keyring containing public keys")
	f.BoolVar(&o.skipRefresh, "skip-refresh", false, "do not refresh the local repository cache")

	return cmd
}

// run runs the full dependency update process.
func (o *dependencyUpdateOptions) run(out io.Writer) error {
	man := &downloader.Manager{
		Out:        out,
		ChartPath:  o.chartpath,
		HelmHome:   o.helmhome,
		Keyring:    o.keyring,
		SkipUpdate: o.skipRefresh,
		Getters:    getter.All(settings),
	}
	if o.verify {
		man.Verify = downloader.VerifyAlways
	}
	if settings.Debug {
		man.Debug = true
	}
	return man.Update()
}
