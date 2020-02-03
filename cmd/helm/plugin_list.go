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
	"strings"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
)

func newPluginListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list installed Helm plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			debug("pluginDirs: %s", settings.PluginsDirectory)
			plugins, err := findPlugins(settings.PluginsDirectory)
			if err != nil {
				return err
			}

			table := uitable.New()
			table.AddRow("NAME", "VERSION", "DESCRIPTION")
			for _, p := range plugins {
				table.AddRow(p.Metadata.Name, p.Metadata.Version, p.Metadata.Description)
			}
			fmt.Fprintln(out, table)
			return nil
		},
	}
	return cmd
}

// Provide dynamic auto-completion for plugin names
func compListPlugins(toComplete string) []string {
	var pNames []string
	plugins, err := findPlugins(settings.PluginsDirectory)
	if err == nil {
		for _, p := range plugins {
			if strings.HasPrefix(p.Metadata.Name, toComplete) {
				pNames = append(pNames, p.Metadata.Name)
			}
		}
	}
	return pNames
}
