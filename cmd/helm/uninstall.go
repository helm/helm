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
	"time"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
)

const uninstallDesc = `
This command takes a release name and uninstalls the release.

It removes all of the resources associated with the last release of the chart
as well as the release history, freeing it up for future use.

Use the '--dry-run' flag to see which releases will be uninstalled without actually
uninstalling them.
`

func newUninstallCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewUninstall(cfg)

	cmd := &cobra.Command{
		Use:        "uninstall RELEASE_NAME [...]",
		Aliases:    []string{"del", "delete", "un"},
		SuggestFor: []string{"remove", "rm"},
		Short:      "uninstall a release",
		Long:       uninstallDesc,
		Args:       require.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			for i := 0; i < len(args); i++ {

				res, err := client.Run(args[i])
				if err != nil {
					return err
				}
				if res != nil && res.Info != "" {
					fmt.Fprintln(out, res.Info)
				}

				fmt.Fprintf(out, "release \"%s\" uninstalled\n", args[i])
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.DryRun, "dry-run", false, "simulate a uninstall")
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "prevent hooks from running during uninstallation")
	f.BoolVar(&client.KeepHistory, "keep-history", false, "remove all associated resources and mark the release as deleted, but retain the release history")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.StringVar(&client.Description, "description", "", "add a custom description")

	return cmd
}
