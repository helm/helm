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
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:   "test [RELEASE]",
		Short: "run tests for a release",
		Long:  releaseTestHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rel, err := client.Run(args[0])
			if err != nil {
				return err
			}

			return outfmt.Write(out, &statusPrinter{rel, settings.Debug})
		},
	}

	f := cmd.Flags()
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")

	return cmd
}
