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

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/installer"
)

type pluginUpdateOptions struct {
	names   []string
	version string
}

func newPluginUpdateCmd(out io.Writer) *cobra.Command {
	o := &pluginUpdateOptions{}

	cmd := &cobra.Command{
		Use:     "update <plugin>...",
		Aliases: []string{"up"},
		Short:   "update one or more Helm plugins",
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
	cmd.Flags().StringVar(&o.version, "version", "", "specify a version constraint. If this is not specified, the latest version is installed")
	return cmd
}

func (o *pluginUpdateOptions) complete(args []string) error {
	if len(args) == 0 {
		return errors.New("please provide plugin name to update")
	}
	o.names = args
	return nil
}

func (o *pluginUpdateOptions) run(out io.Writer) error {
	installer.Debug = settings.Debug
	slog.Debug("loading installed plugins", "path", settings.PluginsDirectory)
	plugins, err := plugin.LoadAll(settings.PluginsDirectory)
	if err != nil {
		return err
	}
	var errorPlugins []error

	for _, name := range o.names {
		if found := findPlugin(plugins, name); found != nil {
			if err := updatePlugin(found, o.version); err != nil {
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
