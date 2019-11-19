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
	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

var outdatedHelp = `
This Command lists all releases which are outdated.

By default, the output is printed in a Table but you can change this behavior
with the '--output' Flag.
`

func newOutdatedCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewList(cfg)
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:     "outdated",
		Short:   "list outdated releases",
		Long:    outdatedHelp,
		Aliases: []string{"od"},
		Args:    require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&client.AllNamespaces, "all-namespaces", "A", false, "list releases across all namespaces")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}
