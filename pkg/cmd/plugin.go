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
	"io"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
)

const pluginHelp = `
Manage client-side Helm plugins.
`

func newPluginCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "install, list, or uninstall Helm plugins",
		Long:  pluginHelp,
	}
	cmd.AddCommand(
		newPluginInstallCmd(out),
		newPluginListCmd(out),
		newPluginUninstallCmd(out),
		newPluginUpdateCmd(out),
		newPluginPackageCmd(out),
		newPluginVerifyCmd(out),
	)
	return cmd
}

// runHook will execute a plugin hook.
func runHook(p plugin.Plugin, event string) error {
	pluginHook, ok := p.(plugin.PluginHook)
	if ok {
		return pluginHook.InvokeHook(event)
	}

	return nil
}
