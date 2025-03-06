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
	"path/filepath"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/plugin"
	"helm.sh/helm/v4/pkg/plugin/installer"
)

type pluginListOptions struct {
	showOutdated bool
}

func newPluginListCmd(out io.Writer) *cobra.Command {
	o := &pluginListOptions{}

	cmd := &cobra.Command{
		Use:               "list",
		Aliases:           []string{"ls"},
		Short:             "list installed Helm plugins",
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, _ []string) error {
			Debug("pluginDirs: %s", settings.PluginsDirectory)
			plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
			if err != nil {
				return err
			}

			table := uitable.New()
			table.AddRow("NAME", "VERSION", "LATEST", "DESCRIPTION")
			for _, p := range plugins {
				latest, err := getLatestVersion(p)
				if err != nil {
					latest = "unknown"
				}

				if !o.showOutdated || (latest != "unknown" && latest != p.Metadata.Version) {
					table.AddRow(p.Metadata.Name, p.Metadata.Version, latest, p.Metadata.Description)
				}
			}
			fmt.Fprintln(out, table)
			return nil
		},
	}

	cmd.Flags().BoolVar(&o.showOutdated, "outdated", false, "show only outdated plugins")

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
func compListPlugins(_ string, ignoredPluginNames []string) []string {
	var pNames []string
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err == nil && len(plugins) > 0 {
		filteredPlugins := filterPlugins(plugins, ignoredPluginNames)
		for _, p := range filteredPlugins {
			pNames = append(pNames, fmt.Sprintf("%s\t%s", p.Metadata.Name, p.Metadata.Usage))
		}
	}
	return pNames
}

// getLatestVersion returns the latest version of a plugin
func getLatestVersion(p *plugin.Plugin) (string, error) {
	exactLocation, err := filepath.EvalSymlinks(p.Dir)
	if err != nil {
		return "", err
	}
	absExactLocation, err := filepath.Abs(exactLocation)
	if err != nil {
		return "", err
	}

	i, err := installer.FindSource(absExactLocation)
	if err != nil {
		return "", err
	}

	return i.GetLatestVersion()
}
