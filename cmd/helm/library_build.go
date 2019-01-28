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

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
)

const libraryBuildDesc = `
Build out the library/ directory from the Chart.lock file.

Build is used to reconstruct a chart's libraries to the state specified in
the lock file. This will not re-negotiate libraries, as 'helm library update'
does.

If no lock file is found, 'helm library build' will mirror the behavior
of 'helm library update'.
`

func newLibraryBuildCmd(out io.Writer) *cobra.Command {
	o := &refBuildOptions{
		chartpath: ".",
	}

	cmd := &cobra.Command{
		Use:   "build CHART",
		Short: "rebuild the library/ directory based on the Chart.lock file",
		Long:  libraryBuildDesc,
		Args:  require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.chartpath = args[0]
			}
			return o.run(out, true)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.verify, "verify", false, "verify the packages against signatures")
	f.StringVar(&o.keyring, "keyring", defaultKeyring(), "keyring containing public keys")

	return cmd
}
