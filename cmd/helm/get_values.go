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

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

var getValuesHelp = `
This command downloads a values file for a given release.
`

type valuesWriter struct {
	vals map[string]interface{}
}

func newGetValuesCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var outfmt output.Format
	client := action.NewGetValues(cfg)

	cmd := &cobra.Command{
		Use:   "values RELEASE_NAME",
		Short: "download the values file for a named release",
		Long:  getValuesHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vals, err := client.Run(args[0])
			if err != nil {
				return err
			}
			return outfmt.Write(out, &valuesWriter{vals})
		},
	}

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "get the named release with revision")
	f.BoolVarP(&client.AllValues, "all", "a", false, "dump all (computed) values")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

func (v valuesWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.AddRow("USER-SUPPLIED VALUES:")
	for k, v := range v.vals {
		table.AddRow(k, v)
	}
	return output.EncodeTable(out, table)
}

func (v valuesWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, v)
}

func (v valuesWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, v)
}
