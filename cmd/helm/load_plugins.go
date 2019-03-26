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
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/pkg/getter"
	"helm.sh/helm/pkg/plugin"
)

// loadPlugins loads plugins into the command list.
//
// This follows a different pattern than the other commands because it has
// to inspect its environment and then add commands to the base command
// as it finds them.
func loadPlugins(baseCmd *cobra.Command, out io.Writer) {

	// If HELM_NO_PLUGINS is set to 1, do not load plugins.
	if os.Getenv("HELM_NO_PLUGINS") == "1" {
		return
	}

	found, err := plugin.FindAll(os.Getenv("PATH"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load plugins: %s", err)
		return
	}

	processParent := func(cmd *cobra.Command, args []string) ([]string, error) {
		k, u := manuallyProcessArgs(args)
		if err := cmd.Parent().ParseFlags(k); err != nil {
			return nil, err
		}
		return u, nil
	}

	// Now we create commands for all of these.
	for _, plug := range found {
		plug := plug
		// skip downloader plugins
		if strings.HasPrefix(plug.Name, getter.PluginDownloaderPrefix) {
			continue
		}

		// pull out the name using the last hyphen. We want the cobra command to be the
		// last keyword of the plugin. For example, `helm-plugin-install` should show up
		// under `helm plugin` as `install`.
		plugName := plug.Name[strings.LastIndex(plug.Name, "-")+1:]

		c := &cobra.Command{
			Use:   plugName,
			Short: fmt.Sprintf("the %q plugin", plugName),
			RunE: func(cmd *cobra.Command, args []string) error {
				u, err := processParent(cmd, args)
				if err != nil {
					return err
				}

				// Call setupEnv before PrepareCommand because
				// PrepareCommand uses os.ExpandEnv and expects the
				// setupEnv vars.
				plugin.SetupPluginEnv(settings, plug.Name, plug.Dir)

				prog := exec.Command(plugin.PluginNamePrefix+plug.Name, u...)
				prog.Env = os.Environ()
				prog.Stdin = os.Stdin
				prog.Stdout = out
				prog.Stderr = os.Stderr
				if err := prog.Run(); err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						os.Stderr.Write(eerr.Stderr)
						return errors.Errorf("plugin %q exited with error", plug.Name)
					}
					return err
				}
				return nil
			},
			// This passes all the flags to the subcommand.
			DisableFlagParsing: true,
		}

		// Check if a command with this name does not already exist. If it does, replace it with the plugin
		cmd, _, err := baseCmd.Find(strings.Split(plug.Name, "-"))
		if err != nil {
			panic(err)
		}

		if cmd == baseCmd {
			// if we're back at the root, then we never found an existing command. In that case, add
			// it to the command tree.
			baseCmd.AddCommand(c)
		} else if cmd.Name() == c.Name() {
			baseCmd.RemoveCommand(cmd)
			baseCmd.AddCommand(c)
		} else {
			cmd.AddCommand(c)
		}
	}
}

// manuallyProcessArgs processes an arg array, removing special args.
//
// Returns two sets of args: known and unknown (in that order)
func manuallyProcessArgs(args []string) ([]string, []string) {
	known := []string{}
	unknown := []string{}
	kvargs := []string{"--context", "--home", "--namespace"}
	knownArg := func(a string) bool {
		for _, pre := range kvargs {
			if strings.HasPrefix(a, pre+"=") {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--debug":
			known = append(known, a)
		case "--context", "--home", "--namespace":
			known = append(known, a, args[i+1])
			i++
		default:
			if knownArg(a) {
				known = append(known, a)
				continue
			}
			unknown = append(unknown, a)
		}
	}
	return known, unknown
}
