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
	"helm.sh/helm/v3/pkg/cli/output"
)

var getValuesHelp = `
This command downloads a values file for a given release.
`

type valuesWriter struct {
	vals      map[string]interface{}
	allValues bool
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
			return outfmt.Write(out, &valuesWriter{vals, client.AllValues})
		},
	}

	// Function providing dynamic auto-completion
	completion.RegisterValidArgsFunc(cmd, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) != 0 {
			return nil, completion.BashCompDirectiveNoFileComp
		}
		return compListReleases(toComplete, cfg)
	})

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "get the named release with revision")
	flag := f.Lookup("revision")
	completion.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) == 1 {
			return compListRevisions(cfg, args[0])
		}
		return nil, completion.BashCompDirectiveNoFileComp
	})
	f.BoolVarP(&client.AllValues, "all", "a", false, "dump all (computed) values")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

func (v valuesWriter) WriteTable(out io.Writer) error {
	if v.allValues {
		fmt.Fprintln(out, "COMPUTED VALUES:")
	} else {
		fmt.Fprintln(out, "USER-SUPPLIED VALUES:")
	}
	return output.EncodeYAML(out, v.vals)
}

func (v valuesWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, v.vals)
}

func (v valuesWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, v.vals)
}
