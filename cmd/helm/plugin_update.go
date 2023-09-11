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
	"errors"
	"fmt"
	"io"
	"path/filepath"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/plugin"
	"helm.sh/helm/v3/pkg/plugin/installer"
)

type pluginUpdateOptions struct {
	plugins []*plugin.Plugin
	all     bool
}

func newPluginUpdateCmd(out io.Writer) *cobra.Command {
	o := &pluginUpdateOptions{}

	cmd := &cobra.Command{
		Use:     "update <plugin>...",
		Aliases: []string{"up"},
		Short:   "update one or more Helm plugins",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListPlugins(toComplete, args), cobra.ShellCompDirectiveNoFileComp
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return o.args(args)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return o.findSelectedPlugins(args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out)
		},
	}
	cmd.Flags().BoolVarP(&o.all, "all", "a", false, "Update all installed plugins")
	return cmd
}

func (o *pluginUpdateOptions) args(args []string) error {
	if (o.all && len(args) > 0) || (!o.all && len(args) == 0) {
		return errors.New("please provide plugin name to update or use --all")
	}
	return nil
}

func (o *pluginUpdateOptions) findSelectedPlugins(args []string) error {
	debug("loading installed plugins from %s", settings.PluginsDirectory)
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err != nil {
		return err
	}

	if o.all {
		o.plugins = plugins
	} else {
		for _, name := range args {
			if plugin := findPlugin(plugins, name); plugin != nil {
				o.plugins = append(o.plugins, plugin)
			} else {
				err = multierror.Append(err, fmt.Errorf("plugin: %s not found", name))
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *pluginUpdateOptions) run(out io.Writer) error {
	installer.Debug = settings.Debug

	var errs error
	for _, plugin := range o.plugins {
		name := plugin.Metadata.Name
		if err := updatePlugin(plugin); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("failed to update plugin %s, got error (%w)", name, err))
		} else {
			fmt.Fprintf(out, "Updated plugin: %s\n", name)
		}
	}
	return errs
}

func updatePlugin(p *plugin.Plugin) error {
	exactLocation, err := filepath.EvalSymlinks(p.Dir)
	if err != nil {
		return err
	}
	absExactLocation, err := filepath.Abs(exactLocation)
	if err != nil {
		return err
	}

	i, err := installer.FindSource(absExactLocation)
	if err != nil {
		return err
	}
	if err := installer.Update(i); err != nil {
		return err
	}

	debug("loading plugin from %s", i.Path())
	updatedPlugin, err := plugin.LoadDir(i.Path())
	if err != nil {
		return err
	}

	return runHook(updatedPlugin, plugin.Update)
}
