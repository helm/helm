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

var getValuesHelp = `
This command downloads a values file for a given release.
`

func newGetValuesCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewGetValues(cfg)

	cmd := &cobra.Command{
		Use:   "values RELEASE_NAME",
		Short: "download the values file for a named release",
		Long:  getValuesHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := client.Run(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(out, res)
			return nil
		},
	}

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "get the named release with revision")
	f.BoolVarP(&client.AllValues, "all", "a", false, "dump all (computed) values")
	return cmd
}
