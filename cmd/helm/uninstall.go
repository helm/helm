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

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
)

const uninstallDesc = `
This command takes a release name, and then uninstalls the release from Kubernetes.
It removes all of the resources associated with the last release of the chart.

Use the '--dry-run' flag to see which releases will be uninstalled without actually
uninstalling them.
`

func newUninstallCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewUninstall(cfg)

	cmd := &cobra.Command{
		Use:        "uninstall RELEASE_NAME [...]",
		Aliases:    []string{"del", "delete", "un"},
		SuggestFor: []string{"remove", "rm"},
		Short:      "given a release name, uninstall the release from Kubernetes",
		Long:       uninstallDesc,
		Args:       require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for i := 0; i < len(args); i++ {

				res, err := client.Run(args[0])
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

	client.AddFlags(cmd.Flags())

	return cmd
}
