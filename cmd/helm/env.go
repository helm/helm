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
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
)

var envHelp = `
Env prints out all the environment information in use by Helm.
`

func newEnvCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "helm client environment information",
		Long:  envHelp,
		Args:  require.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				keys := getSortedEnvVarKeys()
				return keys, cobra.ShellCompDirectiveNoFileComp
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			envVars := settings.EnvVars()

			if len(args) == 0 {
				// Sort the variables by alphabetical order.
				// This allows for a constant output across calls to 'helm env'.
				keys := getSortedEnvVarKeys()

				for _, k := range keys {
					fmt.Fprintf(out, "%s=\"%s\"\n", k, envVars[k])
				}
			} else {
				fmt.Fprintf(out, "%s\n", envVars[args[0]])
			}
		},
	}
	return cmd
}

func getSortedEnvVarKeys() []string {
	envVars := settings.EnvVars()

	var keys []string
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
