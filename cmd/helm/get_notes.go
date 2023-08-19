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
	"log"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
)

var getNotesHelp = `
This command shows notes provided by the chart of a named release.
`

func newGetNotesCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewGet(cfg)

	cmd := &cobra.Command{
		Use:   "notes RELEASE_NAME",
		Short: "download the notes for a named release",
		Long:  getNotesHelp,
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
			if len(res.Info.Notes) > 0 {
				fmt.Fprintf(out, "NOTES:\n%s\n", res.Info.Notes)
			}
			return nil
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

	return cmd
}
