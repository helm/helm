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

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
)

const pullDesc = `
Retrieve a package from a package repository, and download it locally.

This is useful for fetching packages to inspect, modify, or repackage. It can
also be used to perform cryptographic verification of a chart without installing
the chart.

There are options for unpacking the chart after download. This will create a
directory for the chart and uncompress into that directory.

If the --verify flag is specified, the requested chart MUST have a provenance
file, and MUST pass the verification process. Failure in any part of this will
result in an error, and the chart will not be saved locally.
`

func newPullCmd(out io.Writer) *cobra.Command {
	client := action.NewPull()

	cmd := &cobra.Command{
		Use:     "pull [chart URL | repo/chartname] [...]",
		Short:   "download a chart from a repository and (optionally) unpack it in local directory",
		Aliases: []string{"fetch"},
		Long:    pullDesc,
		Args:    require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Settings = settings
			if client.Version == "" && client.Devel {
				debug("setting version to >0.0.0-0")
				client.Version = ">0.0.0-0"
			}

			for i := 0; i < len(args); i++ {
				output, err := client.Run(args[i])
				if err != nil {
					return err
				}
				fmt.Fprint(out, output)
			}
			return nil
		},
	}

	// Function providing dynamic auto-completion
	completion.RegisterValidArgsFunc(cmd, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) != 0 {
			return nil, completion.BashCompDirectiveNoFileComp
		}
		return compListCharts(toComplete, false)
	})

	f := cmd.Flags()
	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.")
	f.BoolVar(&client.Untar, "untar", false, "if set to true, will untar the chart after downloading it")
	f.BoolVar(&client.VerifyLater, "prov", false, "fetch the provenance file, but don't perform verification")
	f.StringVar(&client.UntarDir, "untardir", ".", "if untar is specified, this flag specifies the name of the directory into which the chart is expanded")
	f.StringVarP(&client.DestDir, "destination", "d", ".", "location to write the chart. If this and tardir are specified, tardir is appended to this")
	addChartPathOptionsFlags(f, &client.ChartPathOptions)

	return cmd
}
