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

package cmd

import (
	"io"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/registry"
)

const aliasListDesc = `
List registry aliases and substitutions.
`

func newAliasListCmd(_ *action.Configuration, out io.Writer) *cobra.Command {
	var aliasesOpt, substitutionsOpt bool

	cmd := &cobra.Command{
		Use:               "list",
		Short:             "list aliases and substitutions",
		Long:              aliasListDesc,
		Args:              require.NoArgs,
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, _ []string) error {
			var err error
			a, _ := registry.LoadAliasesFile(settings.RegistryAliasConfig)

			if aliasesOpt || !substitutionsOpt {
				table := uitable.New()
				table.AddRow("ALIAS", "URL")
				for a, url := range a.Aliases {
					table.AddRow(a, url)
				}
				err = output.EncodeTable(out, table)
			}

			if substitutionsOpt || !aliasesOpt {
				table := uitable.New()
				table.AddRow("SUBSTITUTION", "REPLACEMENT")
				for s, r := range a.Substitutions {
					table.AddRow(s, r)
				}
				err = output.EncodeTable(out, table)
			}

			return err
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&aliasesOpt, "aliases", "a", false, "list aliases")
	f.BoolVarP(&substitutionsOpt, "substitutions", "s", false, "list substitutions")

	return cmd
}
