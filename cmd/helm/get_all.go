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
	"log"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

var getAllHelp = `
This command prints a human readable collection of information about the
notes, hooks, supplied values, and generated manifest file of the given release.
`

func newGetAllCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var template string
	client := action.NewGet(cfg)

	cmd := &cobra.Command{
		Use:   "all RELEASE_NAME",
		Short: "download all information for a named release",
		Long:  getAllHelp,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := client.Run(args[0])
			if err != nil {
				return err
			}
			if template != "" {
				data := map[string]interface{}{
					"Release": res,
				}
				return tpl(template, data, out)
			}

			return output.Table.Write(out, &statusPrinter{res, true, false})
		},
	}

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "get the named release with revision")
	err := cmd.RegisterFlagCompletionFunc("revision", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return compListRevisions(toComplete, cfg, args[0])
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	})

	if err != nil {
		log.Fatal(err)
	}

	f.StringVar(&template, "template", "", "go template for formatting the output, eg: {{.Release.Name}}")

	return cmd
}
