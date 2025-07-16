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

package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugins"
	pluginloader "helm.sh/helm/v4/internal/plugins/loader"
	"helm.sh/helm/v4/internal/plugins/runtimes/subprocess"
)

func newPluginListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Aliases:           []string{"ls"},
		Short:             "list installed Helm plugins",
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, _ []string) error {
			slog.Debug("pluginDirs", "directory", settings.PluginsDirectory)
			plugins, err := pluginloader.FindPlugins(
				[]string{settings.PluginsDirectory},
				cliPluginDescriptor)
			if err != nil {
				return err
			}

			table := uitable.New()
			table.AddRow("NAME", "VERSION", "DESCRIPTION")
			for _, p := range plugins {
				sp := p.(*subprocess.Plugin)
				table.AddRow(sp.Metadata.Name, sp.Metadata.Version, sp.Metadata.Description)
			}
			fmt.Fprintln(out, table)
			return nil
		},
	}
	return cmd
}

// Returns all plugins from plugins, except those with names matching ignoredPluginNames
func filterPlugins(ps []plugins.Plugin, ignoredPluginNames []string) []plugins.Plugin {
	// if ignoredPluginNames is nil, just return plugins
	if ignoredPluginNames == nil {
		return ps
	}

	filteredPlugins := make([]plugins.Plugin, 0, len(ps))
	for _, p := range ps {
		sp := p.(*subprocess.Plugin)
		found := slices.Contains(ignoredPluginNames, sp.Metadata.Name)
		if !found {
			filteredPlugins = append(filteredPlugins, sp)
		}
	}

	return filteredPlugins
}

// Provide dynamic auto-completion for plugin names
func compListPlugins(_ string, ignoredPluginNames []string) []string {
	var pNames []string
	plugins, err := pluginloader.FindPlugins(
		[]string{settings.PluginsDirectory},
		cliPluginDescriptor)
	if err == nil && len(plugins) > 0 {
		filteredPlugins := filterPlugins(plugins, ignoredPluginNames)
		for _, p := range filteredPlugins {
			sp := p.(*subprocess.Plugin)
			pNames = append(pNames, fmt.Sprintf("%s\t%s", sp.Metadata.Name, sp.Metadata.Usage))
		}
	}
	return pNames
}
