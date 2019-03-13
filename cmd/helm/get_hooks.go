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

const getHooksHelp = `
This command downloads hooks for a given release.

Hooks are formatted in YAML and separated by the YAML '---\n' separator.
`

func newGetHooksCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewGet(cfg)

	cmd := &cobra.Command{
		Use:   "hooks RELEASE_NAME",
		Short: "download all hooks for a named release",
		Long:  getHooksHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := client.Run(args[0])
			if err != nil {
				return err
			}
			for _, hook := range res.Hooks {
				fmt.Fprintf(out, "---\n# %s\n%s", hook.Name, hook.Manifest)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&client.Version, "revision", 0, "get the named release with revision")

	return cmd
}
