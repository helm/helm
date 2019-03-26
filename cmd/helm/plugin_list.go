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
	"os"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"helm.sh/helm/pkg/plugin"
)

const pluginListHelp = `
List all available plugin files on a user's PATH.

Available plugin files are those that are:
- executable
- anywhere on the user's PATH
- begin with "helm-"
`

type pluginListOptions struct {
}

func newPluginListCmd(out io.Writer) *cobra.Command {
	o := &pluginListOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list all installed Helm plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out)
		},
	}
	return cmd
}

func (o *pluginListOptions) run(out io.Writer) error {
	plugins, err := plugin.FindAll(os.Getenv("PATH"))
	if err != nil {
		return err
	}

	table := uitable.New()
	table.AddRow("NAME", "DIRECTORY")
	for _, p := range plugins {
		table.AddRow(p.Name, p.Dir)
	}
	fmt.Fprintln(out, table)
	return nil
}
