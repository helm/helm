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
	"path/filepath"
	"slices"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/schema"
)

func newPluginListCmd(out io.Writer) *cobra.Command {
	var pluginType string
	cmd := &cobra.Command{
		Use:               "list",
		Aliases:           []string{"ls"},
		Short:             "list installed Helm plugins",
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, _ []string) error {
			slog.Debug("pluginDirs", "directory", settings.PluginsDirectory)
			dirs := filepath.SplitList(settings.PluginsDirectory)
			descriptor := plugin.Descriptor{
				Type: pluginType,
			}
			plugins, err := plugin.FindPlugins(dirs, descriptor)
			if err != nil {
				return err
			}

			// Get signing info for all plugins
			signingInfo := plugin.GetSigningInfoForPlugins(plugins)

			table := uitable.New()
			table.AddRow("NAME", "VERSION", "TYPE", "APIVERSION", "PROVENANCE", "SOURCE")
			for _, p := range plugins {
				m := p.Metadata()
				sourceURL := m.SourceURL
				if sourceURL == "" {
					sourceURL = "unknown"
				}
				// Get signing status
				signedStatus := "unknown"
				if info, ok := signingInfo[m.Name]; ok {
					signedStatus = info.Status
				}
				table.AddRow(m.Name, m.Version, m.Type, m.APIVersion, signedStatus, sourceURL)
			}
			fmt.Fprintln(out, table)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&pluginType, "type", "", "Plugin type")

	return cmd
}

// Returns all plugins from plugins, except those with names matching ignoredPluginNames
func filterPlugins(plugins []plugin.Plugin, ignoredPluginNames []string) []plugin.Plugin {
	// if ignoredPluginNames is nil or empty, just return plugins
	if len(ignoredPluginNames) == 0 {
		return plugins
	}

	var filteredPlugins []plugin.Plugin
	for _, plugin := range plugins {
		found := slices.Contains(ignoredPluginNames, plugin.Metadata().Name)
		if !found {
			filteredPlugins = append(filteredPlugins, plugin)
		}
	}

	return filteredPlugins
}

// Provide dynamic auto-completion for plugin names
func compListPlugins(_ string, ignoredPluginNames []string) []string {
	var pNames []string
	dirs := filepath.SplitList(settings.PluginsDirectory)
	descriptor := plugin.Descriptor{
		Type: "cli/v1",
	}
	plugins, err := plugin.FindPlugins(dirs, descriptor)
	if err == nil && len(plugins) > 0 {
		filteredPlugins := filterPlugins(plugins, ignoredPluginNames)
		for _, p := range filteredPlugins {
			m := p.Metadata()
			var shortHelp string
			if config, ok := m.Config.(*schema.ConfigCLIV1); ok {
				shortHelp = config.ShortHelp
			}
			pNames = append(pNames, fmt.Sprintf("%s\t%s", p.Metadata().Name, shortHelp))
		}
	}
	return pNames
}
