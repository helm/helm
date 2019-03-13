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
	"k8s.io/helm/pkg/action"
)

const verifyDesc = `
Verify that the given chart has a valid provenance file.

Provenance files provide crytographic verification that a chart has not been
tampered with, and was packaged by a trusted provider.

This command can be used to verify a local chart. Several other commands provide
'--verify' flags that run the same validation. To generate a signed package, use
the 'helm package --sign' command.
`

func newVerifyCmd(out io.Writer) *cobra.Command {
	client := action.NewVerify()

	cmd := &cobra.Command{
		Use:   "verify PATH",
		Short: "verify that a chart at the given path has been signed and is valid",
		Long:  verifyDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Run(args[0])
		},
	}

	cmd.Flags().StringVar(&client.Keyring, "keyring", defaultKeyring(), "keyring containing public keys")

	return cmd
}
