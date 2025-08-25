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
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/installer"
)

type pluginUpdateOptions struct {
	names []string
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
	pmc, ok := settings.PluginCatalog.(*plugin.PluginManagerCatalog)
	if !ok {
		return nil
	}
	pm := pmc.Manager

	var errs []error
	for _, name := range o.names {
		pluginRaw := pm.Store.Load(name)
		if pluginRaw == nil {
			errs = append(errs, fmt.Errorf("plugin: %s not found", name))
			continue
		}

		if err := updatePlugin(pm, pluginRaw); err != nil {
			errs = append(errs, fmt.Errorf("failed to update plugin %s, got error (%v)", name, err))
			continue
		}

		fmt.Fprintf(out, "Uninstalled plugin: %s\n", name)
	}

	return errors.Join(errs...)
}

func updatePlugin(pm *plugin.Manager, pluginRaw *plugin.PluginRaw) error {
	exactLocation, err := filepath.EvalSymlinks(pluginRaw.Dir)
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

	pm.Store.Store(pluginRaw)

	return runHook(pm, pluginRaw, plugin.Update)
}
