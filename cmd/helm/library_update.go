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

const libraryUpDesc = `
Update the on-disk libraries to mirror Chart.yaml.

This command verifies that the required charts, as expressed in 'Chart.yaml',
are present in 'library/' and are at an acceptable version. It will pull down
the latest charts that satisfy the libraries, and clean up old libraries.

On successful update, this will generate a lock file that can be used to
rebuild the libraries to an exact version.

Libraries are not required to be represented in 'Chart.yaml'. For that
reason, an update command will not remove charts unless they are (a) present
in the Chart.yaml file, but (b) at the wrong version.
`

// newLibraryUpdateCmd creates a new library update command.
func newLibraryUpdateCmd(out io.Writer) *cobra.Command {
	o := &refUpdateOptions{
		chartpath: ".",
	}

	cmd := &cobra.Command{
		Use:     "update CHART",
		Aliases: []string{"up"},
		Short:   "update library/ based on the contents of Chart.yaml",
		Long:    libraryUpDesc,
		Args:    require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.chartpath = filepath.Clean(args[0])
			}
			o.helmhome = settings.Home
			return o.run(out, true)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.verify, "verify", false, "verify the packages against signatures")
	f.StringVar(&o.keyring, "keyring", defaultKeyring(), "keyring containing public keys")
	f.BoolVar(&o.skipRefresh, "skip-refresh", false, "do not refresh the local repository cache")

	return cmd
}
