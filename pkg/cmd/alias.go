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

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
)

const aliasHelp = `
This command consists of multiple subcommands to interact with OCI aliases.
`

func newAliasCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "manage OCI aliases and substitutions",
		Long:  aliasHelp,
	}
	cmd.AddCommand(
		newAliasListCmd(cfg, out),
		newAliasSetCmd(cfg, out),
		newAliasSubstituteCmd(cfg, out),
	)
	return cmd
}
