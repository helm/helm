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

	"helm.sh/helm/v3/pkg/plugin"
)

func newPluginListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Aliases:           []string{"ls"},
		Short:             "list installed Helm plugins",
		ValidArgsFunction: noCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			debug("pluginDirs: %s", settings.PluginsDirectory)
			plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
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

// Returns all plugins from plugins, except those with names matching ignoredPluginNames
func filterPlugins(plugins []*plugin.Plugin, ignoredPluginNames []string) []*plugin.Plugin {
	// if ignoredPluginNames is nil, just return plugins
	if ignoredPluginNames == nil {
		return plugins
	}

	var filteredPlugins []*plugin.Plugin
	for _, plugin := range plugins {
		found := false
		for _, ignoredName := range ignoredPluginNames {
			if plugin.Metadata.Name == ignoredName {
				found = true
				break
			}
		}
		if !found {
			filteredPlugins = append(filteredPlugins, plugin)
		}
	}

	return filteredPlugins
}

// Provide dynamic auto-completion for plugin names
func compListPlugins(toComplete string, ignoredPluginNames []string) []string {
	var pNames []string
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err == nil && len(plugins) > 0 {
		filteredPlugins := filterPlugins(plugins, ignoredPluginNames)
		for _, p := range filteredPlugins {
			if strings.HasPrefix(p.Metadata.Name, toComplete) {
				pNames = append(pNames, fmt.Sprintf("%s\t%s", p.Metadata.Name, p.Metadata.Usage))
			}
		}
	}
	return pNames
}
