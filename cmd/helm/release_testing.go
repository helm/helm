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
	"helm.sh/helm/v3/pkg/cli/output"
)

const releaseTestHelp = `
The test command runs the tests for a release.

The argument this command takes is the name of a deployed release.
The tests to be run are defined in the chart that was installed.
`

func newReleaseTestCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewReleaseTesting(cfg)
	var outfmt = output.Table
	var outputLogs bool

	cmd := &cobra.Command{
		Use:   "test [RELEASE]",
		Short: "run tests for a release",
		Long:  releaseTestHelp,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListReleases(toComplete, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Namespace = settings.Namespace()
			rel, runErr := client.Run(args[0])
			// We only return an error if we weren't even able to get the
			// release, otherwise we keep going so we can print status and logs
			// if requested
			if runErr != nil && rel == nil {
				return runErr
			}

			if err := outfmt.Write(out, &statusPrinter{rel, settings.Debug, false}); err != nil {
				return err
			}

			if outputLogs {
				// Print a newline to stdout to separate the output
				fmt.Fprintln(out)
				if err := client.GetPodLogs(out, rel); err != nil {
					return err
				}
			}

			return runErr
		},
	}

	f := cmd.Flags()
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&outputLogs, "logs", false, "dump the logs from test pods (this runs after all tests are complete, but before any cleanup)")

	return cmd
}
