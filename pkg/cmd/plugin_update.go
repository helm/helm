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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/installer"
)

type pluginUpdateOptions struct {
	plugins map[string]string
}

const pluginUpdateDesc = `Update one or more Helm plugins.

An exact semver version can be supplied per-plugin using the @version syntax:

    helm plugin update myplugin@1.2.3 otherplugin@2.0.0
    helm plugin update myplugin@1.0.0

Range constraints (e.g. ~1.2, ^1.0.0, >=1.0.0, v1.0.0) are not supported.

If no version is given for a plugin it is updated to the latest version:

    helm plugin update myplugin otherplugin
`

func newPluginUpdateCmd(out io.Writer) *cobra.Command {
	o := &pluginUpdateOptions{}

	cmd := &cobra.Command{
		Use:     "update <plugin[@version]>...",
		Aliases: []string{"up"},
		Short:   "update one or more Helm plugins",
		Long:    pluginUpdateDesc,
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListPlugins(toComplete, args), cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(_ *cobra.Command, args []string) error {
			return o.complete(args)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return o.run(out)
		},
	}
	return cmd
}

func (o *pluginUpdateOptions) complete(args []string) error {
	if len(args) == 0 {
		return errors.New("please provide plugin name to update")
	}

	o.plugins = make(map[string]string, len(args))

	for _, arg := range args {
		name, version := parsePluginVersion(arg)
		if name == "" {
			return fmt.Errorf("invalid plugin reference %q: plugin name must not be empty", arg)
		}
		if _, exists := o.plugins[name]; exists {
			return fmt.Errorf("plugin %q specified more than once", name)
		}
		if version != "" {
			if _, err := semver.StrictNewVersion(version); err != nil {
				return fmt.Errorf("invalid version %q for plugin %q: must be an exact semver version (e.g. 1.2.3); the 'v' prefix is not allowed", version, name)
			}
		}
		o.plugins[name] = version
	}

	return nil
}

func (o *pluginUpdateOptions) run(out io.Writer) error {
	slog.Debug("loading installed plugins", "path", settings.PluginsDirectory)
	installed, err := plugin.LoadAll(settings.PluginsDirectory)
	if err != nil {
		return err
	}
	var errorPlugins []error

	for name, version := range o.plugins {
		if found := findPlugin(installed, name); found != nil {
			if err := updatePlugin(found, version); err != nil {
				errorPlugins = append(errorPlugins, fmt.Errorf("failed to update plugin %s, got error (%v)", name, err))
			} else {
				fmt.Fprintf(out, "Updated plugin: %s\n", name)
			}
		} else {
			errorPlugins = append(errorPlugins, fmt.Errorf("plugin: %s not found", name))
		}
	}
	if len(errorPlugins) > 0 {
		return errors.Join(errorPlugins...)
	}
	return nil
}

func parsePluginVersion(arg string) (name, version string) {
	if i := strings.LastIndex(arg, "@"); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}

func updatePlugin(p plugin.Plugin, version string) error {
	exactLocation, err := filepath.EvalSymlinks(p.Dir())
	if err != nil {
		return err
	}
	absExactLocation, err := filepath.Abs(exactLocation)
	if err != nil {
		return err
	}

	i, err := installer.FindSource(absExactLocation, version)
	if err != nil {
		return err
	}
	if err := installer.Update(i); err != nil {
		return err
	}

	slog.Debug("loading plugin", "path", i.Path())
	updatedPlugin, err := plugin.LoadDir(i.Path())
	if err != nil {
		return err
	}

	return runHook(updatedPlugin, plugin.Update)
}
