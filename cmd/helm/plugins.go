/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/helmpath"
)

const pluginEnvVar = "HELM_PLUGIN"

// Plugin describes a plugin.
type Plugin struct {
	// Name is the name of the plugin
	Name string `json:"name"`

	// Version is a SemVer 2 version of the plugin.
	Version string `json:"version"`

	// Usage is the single-line usage text shown in help
	Usage string `json:"usage"`

	// Description is a long description shown in places like `helm help`
	Description string `json:"description"`

	// Command is the command, as a single string.
	//
	// The command will be passed through environment expansion, so env vars can
	// be present in this command. Unless IgnoreFlags is set, this will
	// also merge the flags passed from Helm.
	//
	// Note that command is not executed in a shell. To do so, we suggest
	// pointing the command to a shell script.
	Command string `json:"command"`

	// IgnoreFlags ignores any flags passed in from Helm
	//
	// For example, if the plugin is invoked as `helm --debug myplugin`, if this
	// is false, `--debug` will be appended to `--command`. If this is true,
	// the `--debug` flag will be discarded.
	IgnoreFlags bool `json:"ignoreFlags"`

	// UseTunnel indicates that this command needs a tunnel.
	// Setting this will cause a number of side effects, such as the
	// automatic setting of HELM_HOST.
	UseTunnel bool `json:"useTunnel"`
}

// loadPlugins loads plugins into the command list.
//
// This follows a different pattern than the other commands because it has
// to inspect its environment and then add commands to the base command
// as it finds them.
func loadPlugins(baseCmd *cobra.Command, home helmpath.Home, out io.Writer) {
	plugdirs := os.Getenv(pluginEnvVar)
	if plugdirs == "" {
		plugdirs = home.Plugins()
	}
	found := findPlugins(plugdirs)

	// Now we create commands for all of these.
	for _, path := range found {

		data, err := ioutil.ReadFile(path)
		if err != nil {
			// What should we do here? For now, write a message.
			fmt.Fprintf(os.Stderr, "Failed to load %s: %s (skipping)", path, err)
			continue
		}

		plug := &Plugin{}
		if err := yaml.Unmarshal(data, plug); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse %s: %s (skipping)", path, err)
			continue
		}

		base := filepath.Base(path)
		if plug.Usage == "" {
			plug.Usage = fmt.Sprintf("the %q plugin", base)
		}

		c := &cobra.Command{
			Use:   plug.Name,
			Short: plug.Usage,
			Long:  plug.Description,
			RunE: func(cmd *cobra.Command, args []string) error {
				setupEnv(plug.Name, base, plugdirs, home)
				main, argv := plug.prepareCommand(args)
				prog := exec.Command(main, argv...)
				prog.Stdout = out
				return prog.Run()
			},
			// This passes all the flags to the subcommand.
			DisableFlagParsing: true,
		}

		if plug.UseTunnel {
			c.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
				// Parse the parent flag, but not the local flags.
				c.Parent().ParseFlags(args)
				return setupConnection(cmd, args)
			}
		}

		// TODO: Make sure a command with this name does not already exist.
		baseCmd.AddCommand(c)
	}
}

func (p *Plugin) prepareCommand(extraArgs []string) (string, []string) {
	parts := strings.Split(os.ExpandEnv(p.Command), " ")
	main := parts[0]
	baseArgs := []string{}
	if len(parts) > 1 {
		baseArgs = parts[1:]
	}
	if !p.IgnoreFlags {
		baseArgs = append(baseArgs, extraArgs...)
	}
	return main, baseArgs
}

// findPlugins returns a list of YAML files that describe plugins.
func findPlugins(plugdirs string) []string {
	found := []string{}
	// Let's get all UNIXy and allow path separators
	for _, p := range filepath.SplitList(plugdirs) {
		// Look for any commands that match the Helm plugin pattern:
		matches, err := filepath.Glob(filepath.Join(p, "*.yaml"))
		if err != nil {
			continue
		}
		found = append(found, matches...)
	}
	return found
}

func setupEnv(shortname, base, plugdirs string, home helmpath.Home) {
	// Set extra env vars:
	for key, val := range map[string]string{
		"HELM_PLUGIN_SHORTNAME": shortname,
		"HELM_PLUGIN_NAME":      base,
		"HELM_BIN":              os.Args[0],

		// Set vars that may not have been set, and save client the
		// trouble of re-parsing.
		pluginEnvVar: plugdirs,
		homeEnvVar:   home.String(),

		// Set vars that convey common information.
		"HELM_PATH_REPOSITORY":       home.Repository(),
		"HELM_PATH_REPOSITORY_FILE":  home.RepositoryFile(),
		"HELM_PATH_CACHE":            home.Cache(),
		"HELM_PATH_LOCAL_REPOSITORY": home.LocalRepository(),

		// TODO: Add helm starter packs var when that is merged.
	} {
		os.Setenv(key, val)
	}

}
