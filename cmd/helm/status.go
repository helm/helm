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
	"encoding/json"
	"io"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
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
			rel, err := client.Run(args[0])
			if err != nil {
				return err
			}

			// strip chart metadata from the output
			rel.Chart = nil

			outfmt, err := action.ParseOutputFormat(client.OutputFormat)
			// We treat an invalid format type as the default
			if err != nil && err != action.ErrInvalidFormatType {
				return err
			}

			switch outfmt {
			case "":
				action.PrintRelease(out, rel)
				return nil
			case action.JSON:
				data, err := json.Marshal(rel)
				if err != nil {
					return errors.Wrap(err, "failed to Marshal JSON output")
				}
				out.Write(data)
				return nil
			case action.YAML:
				data, err := yaml.Marshal(rel)
				if err != nil {
					return errors.Wrap(err, "failed to Marshal YAML output")
				}
				out.Write(data)
				return nil
			default:
				return errors.Errorf("unknown output format %q", outfmt)
			}
		},
	}

	f := cmd.PersistentFlags()
	f.IntVar(&client.Version, "revision", 0, "if set, display the status of the named release with revision")
	f.StringVarP(&client.OutputFormat, "output", "o", "", "output the status in the specified format (json or yaml)")

	return cmd
}
