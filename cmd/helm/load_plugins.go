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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/plugin"
)

const pluginStaticCompletionFile = "completion.yaml"

type pluginError struct {
	error
	code int
}

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

	found, err := findPlugins(settings.PluginsDirectory)
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

	// If we are dealing with the completion command, we try to load more details about the plugins
	// if available, so as to allow for command and flag completion
	if subCmd, _, err := baseCmd.Find(os.Args[1:]); err == nil && subCmd.Name() == "completion" {
		loadPluginsForCompletion(baseCmd, found)
		return
	}

	// Now we create commands for all of these.
	for _, plug := range found {
		plug := plug
		md := plug.Metadata
		if md.Usage == "" {
			md.Usage = fmt.Sprintf("the %q plugin", md.Name)
		}

		c := &cobra.Command{
			Use:   md.Name,
			Short: md.Usage,
			Long:  md.Description,
			RunE: func(cmd *cobra.Command, args []string) error {
				u, err := processParent(cmd, args)
				if err != nil {
					return err
				}

				// Call setupEnv before PrepareCommand because
				// PrepareCommand uses os.ExpandEnv and expects the
				// setupEnv vars.
				plugin.SetupPluginEnv(settings, md.Name, plug.Dir)
				main, argv, prepCmdErr := plug.PrepareCommand(u)
				if prepCmdErr != nil {
					os.Stderr.WriteString(prepCmdErr.Error())
					return errors.Errorf("plugin %q exited with error", md.Name)
				}

				env := os.Environ()
				for k, v := range settings.EnvVars() {
					env = append(env, fmt.Sprintf("%s=%s", k, v))
				}

				prog := exec.Command(main, argv...)
				prog.Env = env
				prog.Stdin = os.Stdin
				prog.Stdout = out
				prog.Stderr = os.Stderr
				if err := prog.Run(); err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						os.Stderr.Write(eerr.Stderr)
						status := eerr.Sys().(syscall.WaitStatus)
						return pluginError{
							error: errors.Errorf("plugin %q exited with error", md.Name),
							code:  status.ExitStatus(),
						}
					}
					return err
				}
				return nil
			},
			// This passes all the flags to the subcommand.
			DisableFlagParsing: true,
		}

		// TODO: Make sure a command with this name does not already exist.
		baseCmd.AddCommand(c)
	}
}

// manuallyProcessArgs processes an arg array, removing special args.
//
// Returns two sets of args: known and unknown (in that order)
func manuallyProcessArgs(args []string) ([]string, []string) {
	known := []string{}
	unknown := []string{}
	kvargs := []string{"--kube-context", "--namespace", "-n", "--kubeconfig", "--registry-config", "--repository-cache", "--repository-config"}
	knownArg := func(a string) bool {
		for _, pre := range kvargs {
			if strings.HasPrefix(a, pre+"=") {
				return true
			}
		}
		return false
	}

	isKnown := func(v string) string {
		for _, i := range kvargs {
			if i == v {
				return v
			}
		}
		return ""
	}

	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--debug":
			known = append(known, a)
		case isKnown(a):
			known = append(known, a)
			i++
			if i < len(args) {
				known = append(known, args[i])
			}
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

// findPlugins returns a list of YAML files that describe plugins.
func findPlugins(plugdirs string) ([]*plugin.Plugin, error) {
	found := []*plugin.Plugin{}
	// Let's get all UNIXy and allow path separators
	for _, p := range filepath.SplitList(plugdirs) {
		matches, err := plugin.LoadAll(p)
		if err != nil {
			return matches, err
		}
		found = append(found, matches...)
	}
	return found, nil
}

// pluginCommand represents the optional completion.yaml file of a plugin
type pluginCommand struct {
	Name      string          `json:"name"`
	ValidArgs []string        `json:"validArgs"`
	Flags     []string        `json:"flags"`
	Commands  []pluginCommand `json:"commands"`
}

// loadPluginsForCompletion will load and parse any completion.yaml provided by the plugins
func loadPluginsForCompletion(baseCmd *cobra.Command, plugins []*plugin.Plugin) {
	for _, plug := range plugins {
		// Parse the yaml file providing the plugin's subcmds and flags
		cmds, err := loadFile(strings.Join(
			[]string{plug.Dir, pluginStaticCompletionFile}, string(filepath.Separator)))

		if err != nil {
			// The file could be missing or invalid.  Either way, we at least create the command
			// for the plugin name.
			if settings.Debug {
				log.Output(2, fmt.Sprintf("[info] %s\n", err.Error()))
			}
			cmds = &pluginCommand{Name: plug.Metadata.Name}
		}

		// We know what the plugin name must be.
		// Let's set it in case the Name field was not specified correctly in the file.
		// This insures that we will at least get the plugin name to complete, even if
		// there is a problem with the completion.yaml file
		cmds.Name = plug.Metadata.Name

		addPluginCommands(baseCmd, cmds)
	}
}

// addPluginCommands is a recursive method that adds the different levels
// of sub-commands and flags for the plugins that provide such information
func addPluginCommands(baseCmd *cobra.Command, cmds *pluginCommand) {
	if cmds == nil {
		return
	}

	if len(cmds.Name) == 0 {
		// Missing name for a command
		if settings.Debug {
			log.Output(2, fmt.Sprintf("[info] sub-command name field missing for %s", baseCmd.CommandPath()))
		}
		return
	}

	// Create a fake command just so the completion script will include it
	c := &cobra.Command{
		Use:       cmds.Name,
		ValidArgs: cmds.ValidArgs,
		// A Run is required for it to be a valid command without subcommands
		Run: func(cmd *cobra.Command, args []string) {},
	}
	baseCmd.AddCommand(c)

	// Create fake flags.
	if len(cmds.Flags) > 0 {
		// The flags can be created with any type, since we only need them for completion.
		// pflag does not allow to create short flags without a corresponding long form
		// so we look for all short flags and match them to any long flag.  This will allow
		// plugins to provide short flags without a long form.
		// If there are more short-flags than long ones, we'll create an extra long flag with
		// the same single letter as the short form.
		shorts := []string{}
		longs := []string{}
		for _, flag := range cmds.Flags {
			if len(flag) == 1 {
				shorts = append(shorts, flag)
			} else {
				longs = append(longs, flag)
			}
		}

		f := c.Flags()
		if len(longs) >= len(shorts) {
			for i := range longs {
				if i < len(shorts) {
					f.BoolP(longs[i], shorts[i], false, "")
				} else {
					f.Bool(longs[i], false, "")
				}
			}
		} else {
			for i := range shorts {
				if i < len(longs) {
					f.BoolP(longs[i], shorts[i], false, "")
				} else {
					// Create a long flag with the same name as the short flag.
					// Not a perfect solution, but its better than ignoring the extra short flags.
					f.BoolP(shorts[i], shorts[i], false, "")
				}
			}
		}
	}

	// Recursively add any sub-commands
	for _, cmd := range cmds.Commands {
		addPluginCommands(c, &cmd)
	}
}

// loadFile takes a yaml file at the given path, parses it and returns a pluginCommand object
func loadFile(path string) (*pluginCommand, error) {
	cmds := new(pluginCommand)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return cmds, errors.New(fmt.Sprintf("File (%s) not provided by plugin. No plugin auto-completion possible.", path))
	}

	err = yaml.Unmarshal(b, cmds)
	return cmds, err
}
