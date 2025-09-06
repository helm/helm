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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
)

type pluginUninstallOptions struct {
	names []string
}

func newPluginUninstallCmd(out io.Writer) *cobra.Command {
	o := &pluginUninstallOptions{}

	cmd := &cobra.Command{
		Use:     "uninstall <plugin>...",
		Aliases: []string{"rm", "remove"},
		Short:   "uninstall one or more Helm plugins",
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

func (o *pluginUninstallOptions) complete(args []string) error {
	if len(args) == 0 {
		return errors.New("please provide plugin name to uninstall")
	}
	o.names = args
	return nil
}

func (o *pluginUninstallOptions) run(out io.Writer) error {
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

		if err := uninstallPlugin(pm, pluginRaw); err != nil {
			errs = append(errs, fmt.Errorf("failed to uninstall plugin %s, got error (%v)", name, err))
			continue
		}

		fmt.Fprintf(out, "Uninstalled plugin: %s\n", name)
	}

	return errors.Join(errs...)
}

func uninstallPlugin(pm *plugin.Manager, pluginRaw *plugin.PluginRaw) error {
	pm.Store.Delete(pluginRaw.Metadata.Name)

	if err := os.RemoveAll(pluginRaw.Dir); err != nil {
		return err
	}

	// Clean up versioned tarball and provenance files from HELM_PLUGINS directory
	// These files are saved with pattern: PLUGIN_NAME-VERSION.tgz and PLUGIN_NAME-VERSION.tgz.prov
	pluginName := pluginRaw.Metadata.Name
	pluginVersion := pluginRaw.Metadata.Version
	pluginsDir := settings.PluginsDirectory

	// Remove versioned files: plugin-name-version.tgz and plugin-name-version.tgz.prov
	if pluginVersion != "" {
		versionedBasename := fmt.Sprintf("%s-%s.tgz", pluginName, pluginVersion)

		// Remove tarball file
		tarballPath := filepath.Join(pluginsDir, versionedBasename)
		if _, err := os.Stat(tarballPath); err == nil {
			slog.Debug("removing versioned tarball", "path", tarballPath)
			if err := os.Remove(tarballPath); err != nil {
				slog.Debug("failed to remove tarball file", "path", tarballPath, "error", err)
			}
		}

		// Remove provenance file
		provPath := filepath.Join(pluginsDir, versionedBasename+".prov")
		if _, err := os.Stat(provPath); err == nil {
			slog.Debug("removing versioned provenance", "path", provPath)
			if err := os.Remove(provPath); err != nil {
				slog.Debug("failed to remove provenance file", "path", provPath, "error", err)
			}
		}
	}

	// Ensure a concurrent store reload doesn't accidentally race the os.RemoveAll and read the plugin back into memory
	pm.Store.Delete(pluginRaw.Metadata.Name)

	// TODO: should the hook be run before deleting the plugin's files?
	return runHook(pm, pluginRaw, plugin.Delete)
}
