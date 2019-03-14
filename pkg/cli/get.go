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

package cli

import (
	"io"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/action"
	"k8s.io/helm/pkg/cli/require"
)

var getHelp = `
This command shows the details of a named release.

It can be used to get extended information about the release, including:

  - The values used to generate the release
  - The chart used to generate the release
  - The generated manifest file

By default, this prints a human readable collection of information about the
chart, the supplied values, and the generated manifest file.
`

func NewGetCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewGet(cfg)

	cmd := &cobra.Command{
		Use:   "get RELEASE_NAME",
		Short: "download a named release",
		Long:  getHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := client.Run(args[0])
			if err != nil {
				return err
			}
			return printRelease(out, res)
		},
	}

	cmd.Flags().IntVar(&client.Version, "revision", 0, "get the named release with revision")

	cmd.AddCommand(NewGetValuesCmd(cfg, out))
	cmd.AddCommand(NewGetManifestCmd(cfg, out))
	cmd.AddCommand(NewGetHooksCmd(cfg, out))

	return cmd
}
