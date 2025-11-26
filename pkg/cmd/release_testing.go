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
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
)

const releaseTestHelp = `
The test command runs the tests for a release.

The argument this command takes is the name of a deployed release.
The tests to be run are defined in the chart that was installed.
`

func newReleaseTestCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewReleaseTesting(cfg)
	outfmt := output.Table
	var outputLogs bool
	var filter []string

	cmd := &cobra.Command{
		Use:   "test [RELEASE]",
		Short: "run tests for a release",
		Long:  releaseTestHelp,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return noMoreArgsComp()
			}
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(_ *cobra.Command, args []string) (returnError error) {
			client.Namespace = settings.Namespace()
			notName := regexp.MustCompile(`^!\s?name=`)
			for _, f := range filter {
				if after, ok := strings.CutPrefix(f, "name="); ok {
					client.Filters[action.IncludeNameFilter] = append(client.Filters[action.IncludeNameFilter], after)
				} else if notName.MatchString(f) {
					client.Filters[action.ExcludeNameFilter] = append(client.Filters[action.ExcludeNameFilter], notName.ReplaceAllLiteralString(f, ""))
				}
			}

			reli, shutdown, runErr := client.Run(args[0])
			defer func() {
				if shutdownErr := shutdown(); shutdownErr != nil {
					if returnError == nil {
						returnError = shutdownErr
					}
				}
			}()

			// We only return an error if we weren't even able to get the
			// release, otherwise we keep going so we can print status and logs
			// if requested
			if runErr != nil && reli == nil {
				return runErr
			}
			rel, err := releaserToV1Release(reli)
			if err != nil {
				return err
			}

			if err := outfmt.Write(out, &statusPrinter{
				release:      rel,
				debug:        settings.Debug,
				showMetadata: false,
				hideNotes:    true,
				noColor:      settings.ShouldDisableColor(),
			}); err != nil {
				return err
			}

			if outputLogs {
				// Print a newline to stdout to separate the output
				fmt.Fprintln(out)
				if err := client.GetPodLogs(out, rel); err != nil {
					return errors.Join(runErr, err)
				}
			}

			return runErr
		},
	}

	f := cmd.Flags()
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&outputLogs, "logs", false, "dump the logs from test pods (this runs after all tests are complete, but before any cleanup)")
	f.StringSliceVar(&filter, "filter", []string{}, "specify tests by attribute (currently \"name\") using attribute=value syntax or '!attribute=value' to exclude a test (can specify multiple or separate values with commas: name=test1,name=test2)")

	return cmd
}
