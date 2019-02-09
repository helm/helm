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

var listHelp = `
This command lists all of the releases.

By default, it lists only releases that are deployed or failed. Flags like
'--uninstalled' and '--all' will alter this behavior. Such flags can be combined:
'--uninstalled --failed'.

By default, items are sorted alphabetically. Use the '-d' flag to sort by
release date.

If the --filter flag is provided, it will be treated as a filter. Filters are
regular expressions (Perl compatible) that are applied to the list of releases.
Only items that match the filter will be returned.

	$ helm list --filter 'ara[a-z]+'
	NAME            	UPDATED                 	CHART
	maudlin-arachnid	Mon May  9 16:07:08 2016	alpine-0.1.0

If no results are found, 'helm list' will exit 0, but with no output (or in
the case of no '-q' flag, only headers).

By default, up to 256 items may be returned. To limit this, use the '--max' flag.
Setting '--max' to 0 will not return all results. Rather, it will return the
server's default, which may be much higher than 256. Pairing the '--max'
flag with the '--offset' flag allows you to page through results.
`

func newListCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewList(cfg)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list releases",
		Long:    listHelp,
		Aliases: []string{"ls"},
		Args:    require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client.AllNamespaces {
				client.SetConfiguration(newActionConfig(true))
			}
			client.All = client.Limit == -1
			client.SetStateMask()

			results, err := client.Run()

			if client.Short {
				for _, res := range results {
					fmt.Fprintln(out, res.Name)
				}
				return err
			}

			fmt.Fprintln(out, action.FormatList(results))
			return err
		},
	}

	client.AddFlags(cmd.Flags())

	return cmd
}
