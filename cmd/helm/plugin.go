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
	"io"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/plugin"
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
	)
	return cmd
}

// runHook will execute a plugin hook.
func runHook(p *plugin.Plugin, event string) error {
	var cmd string
	var cmdArgs []string

	plugin.SetupPluginEnv(settings, p.Metadata.Name, p.Dir)

	command := p.Metadata.Hooks[event]
	if command != "" {
		cmd = "sh"
		cmdArgs = []string{"-c", command}
	}

	main, argv, err := plugin.PrepareCommands(p.Metadata.PlatformHooks[event], cmd, cmdArgs, false, []string{})
	if err != nil {
		return nil
	}

	prog := exec.Command(main, argv...)

	debug("running %s hook: %s", event, prog)

	prog.Stdout, prog.Stderr = os.Stdout, os.Stderr
	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return errors.Errorf("plugin %s hook for %q exited with error", event, p.Metadata.Name)
		}
		return err
	}
	return nil
}
