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

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

var statusHelp = `
This command shows the status of a named release.
The status consists of:
- last deployment time
- k8s namespace in which the release lives
- state of the release (can be: unknown, deployed, deleted, superseded, failed or deleting)
- list of resources that this release consists of, sorted by kind
- details on last test suite run, if applicable
- additional notes provided by the chart
`

func newStatusCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewStatus(cfg)

	cmd := &cobra.Command{
		Use:   "status RELEASE_NAME",
		Short: "displays the status of the named release",
		Long:  statusHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate the output format first so we don't waste time running a
			// request that we'll throw away
			outfmt, err := action.ParseOutputFormat(client.OutputFormat)
			if err != nil {
				return err
			}

			rel, err := client.Run(args[0])
			if err != nil {
				return err
			}

			// strip chart metadata from the output
			rel.Chart = nil

			return outfmt.Write(out, &statusPrinter{rel, false})
		},
	}

	f := cmd.PersistentFlags()
	f.IntVar(&client.Version, "revision", 0, "if set, display the status of the named release with revision")
	bindOutputFlag(cmd, &client.OutputFormat)

	return cmd
}

type statusPrinter struct {
	release *release.Release
	debug   bool
}

func (s statusPrinter) WriteJSON(out io.Writer) error {
	return action.EncodeJSON(out, s.release)
}

func (s statusPrinter) WriteYAML(out io.Writer) error {
	return action.EncodeYAML(out, s.release)
}

func (s statusPrinter) WriteTable(out io.Writer) error {
	return action.PrintRelease(out, s.release, s.debug)
}
