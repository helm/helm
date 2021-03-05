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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/plugin"
)

const (
	pluginStaticCompletionFile        = "completion.yaml"
	pluginDynamicCompletionExecutable = "plugin.complete"
)

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

	found, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load plugins: %s\n", err)
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

				return callPluginExecutable(md.Name, main, argv, out)
			},
			// This passes all the flags to the subcommand.
			DisableFlagParsing: true,
		}

		// TODO: Make sure a command with this name does not already exist.
		baseCmd.AddCommand(c)

		// For completion, we try to load more details about the plugins so as to allow for command and
		// flag completion of the plugin itself.
		// We only do this when necessary (for the "completion" and "__complete" commands) to avoid the
		// risk of a rogue plugin affecting Helm's normal behavior.
		subCmd, _, err := baseCmd.Find(os.Args[1:])
		if (err == nil &&
			((subCmd.HasParent() && subCmd.Parent().Name() == "completion") || subCmd.Name() == cobra.ShellCompRequestCmd)) ||
			/* for the tests */ subCmd == baseCmd.Root() {
			loadCompletionForPlugin(c, plug)
		}
	}
}

func processParent(cmd *cobra.Command, args []string) ([]string, error) {
	k, u := manuallyProcessArgs(args)
	if err := cmd.Parent().ParseFlags(k); err != nil {
		return nil, err
	}
	return u, nil
}

// This function is used to setup the environment for the plugin and then
// call the executable specified by the parameter 'main'
func callPluginExecutable(pluginName string, main string, argv []string, out io.Writer) error {
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
				error: errors.Errorf("plugin %q exited with error", pluginName),
				code:  status.ExitStatus(),
			}
		}
		return err
	}
	return nil
}

// manuallyProcessArgs processes an arg array, removing special args.
//
// Returns two sets of args: known and unknown (in that order)
func manuallyProcessArgs(args []string) ([]string, []string) {
	known := []string{}
	unknown := []string{}
	kvargs := []string{"--kube-context", "--namespace", "-n", "--kubeconfig", "--kube-apiserver", "--kube-token", "--kube-as-user", "--kube-as-group", "--kube-ca-file", "--registry-config", "--repository-cache", "--repository-config"}
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

// pluginCommand represents the optional completion.yaml file of a plugin
type pluginCommand struct {
	Name      string          `json:"name"`
	ValidArgs []string        `json:"validArgs"`
	Flags     []string        `json:"flags"`
	Commands  []pluginCommand `json:"commands"`
}

// loadCompletionForPlugin will load and parse any completion.yaml provided by the plugin
// and add the dynamic completion hook to call the optional plugin.complete
func loadCompletionForPlugin(pluginCmd *cobra.Command, plugin *plugin.Plugin) {
	// Parse the yaml file providing the plugin's sub-commands and flags
	cmds, err := loadFile(strings.Join(
		[]string{plugin.Dir, pluginStaticCompletionFile}, string(filepath.Separator)))

	if err != nil {
		// The file could be missing or invalid.  No static completion for this plugin.
		if settings.Debug {
			log.Output(2, fmt.Sprintf("[info] %s\n", err.Error()))
		}
		// Continue to setup dynamic completion.
		cmds = &pluginCommand{}
	}

	// Preserve the Usage string specified for the plugin
	cmds.Name = pluginCmd.Use

	addPluginCommands(plugin, pluginCmd, cmds)
}

// addPluginCommands is a recursive method that adds each different level
// of sub-commands and flags for the plugins that have provided such information
func addPluginCommands(plugin *plugin.Plugin, baseCmd *cobra.Command, cmds *pluginCommand) {
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

	baseCmd.Use = cmds.Name
	baseCmd.ValidArgs = cmds.ValidArgs
	// Setup the same dynamic completion for each plugin sub-command.
	// This is because if dynamic completion is triggered, there is a single executable
	// to call (plugin.complete), so every sub-commands calls it in the same fashion.
	if cmds.Commands == nil {
		// Only setup dynamic completion if there are no sub-commands.  This avoids
		// calling plugin.complete at every completion, which greatly simplifies
		// development of plugin.complete for plugin developers.
		baseCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return pluginDynamicComp(plugin, cmd, args, toComplete)
		}
	}

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

		f := baseCmd.Flags()
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
		// Create a fake command so that completion can be done for the sub-commands of the plugin
		subCmd := &cobra.Command{
			// This prevents Cobra from removing the flags.  We want to keep the flags to pass them
			// to the dynamic completion script of the plugin.
			DisableFlagParsing: true,
			// A Run is required for it to be a valid command without subcommands
			Run: func(cmd *cobra.Command, args []string) {},
		}
		baseCmd.AddCommand(subCmd)
		addPluginCommands(plugin, subCmd, &cmd)
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

// pluginDynamicComp call the plugin.complete script of the plugin (if available)
// to obtain the dynamic completion choices.  It must pass all the flags and sub-commands
// specified in the command-line to the plugin.complete executable (except helm's global flags)
func pluginDynamicComp(plug *plugin.Plugin, cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	md := plug.Metadata

	u, err := processParent(cmd, args)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// We will call the dynamic completion script of the plugin
	main := strings.Join([]string{plug.Dir, pluginDynamicCompletionExecutable}, string(filepath.Separator))

	// We must include all sub-commands passed on the command-line.
	// To do that, we pass-in the entire CommandPath, except the first two elements
	// which are 'helm' and 'pluginName'.
	argv := strings.Split(cmd.CommandPath(), " ")[2:]
	if !md.IgnoreFlags {
		argv = append(argv, u...)
		argv = append(argv, toComplete)
	}
	plugin.SetupPluginEnv(settings, md.Name, plug.Dir)

	cobra.CompDebugln(fmt.Sprintf("calling %s with args %v", main, argv), settings.Debug)
	buf := new(bytes.Buffer)
	if err := callPluginExecutable(md.Name, main, argv, buf); err != nil {
		// The dynamic completion file is optional for a plugin, so this error is ok.
		cobra.CompDebugln(fmt.Sprintf("Unable to call %s: %v", main, err.Error()), settings.Debug)
		return nil, cobra.ShellCompDirectiveDefault
	}

	var completions []string
	for _, comp := range strings.Split(buf.String(), "\n") {
		// Remove any empty lines
		if len(comp) > 0 {
			completions = append(completions, comp)
		}
	}

	// Check if the last line of output is of the form :<integer>, which
	// indicates the BashCompletionDirective.
	directive := cobra.ShellCompDirectiveDefault
	if len(completions) > 0 {
		lastLine := completions[len(completions)-1]
		if len(lastLine) > 1 && lastLine[0] == ':' {
			if strInt, err := strconv.Atoi(lastLine[1:]); err == nil {
				directive = cobra.ShellCompDirective(strInt)
				completions = completions[:len(completions)-1]
			}
		}
	}

	return completions, directive
}
