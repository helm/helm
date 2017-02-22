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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/plugin"
)

const pluginEnvVar = "HELM_PLUGIN"

// loadPlugins loads plugins into the command list.
//
// This follows a different pattern than the other commands because it has
// to inspect its environment and then add commands to the base command
// as it finds them.
func loadPlugins(baseCmd *cobra.Command, home helmpath.Home, out io.Writer) {

	// If HELM_NO_PLUGINS is set to 1, do not load plugins.
	if os.Getenv("HELM_NO_PLUGINS") == "1" {
		return
	}

	plugdirs := os.Getenv(pluginEnvVar)
	if plugdirs == "" {
		plugdirs = home.Plugins()
	}

	found, err := findPlugins(plugdirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load plugins: %s", err)
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

				k, u := manuallyProcessArgs(args)
				if err := cmd.Parent().ParseFlags(k); err != nil {
					return err
				}

				// Call setupEnv before PrepareCommand because
				// PrepareCommand uses os.ExpandEnv and expects the
				// setupEnv vars.
				setupEnv(md.Name, plug.Dir, plugdirs, home)
				main, argv := plug.PrepareCommand(u)

				prog := exec.Command(main, argv...)
				prog.Env = os.Environ()
				prog.Stdout = out
				prog.Stderr = os.Stderr
				if err := prog.Run(); err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						os.Stderr.Write(eerr.Stderr)
						return fmt.Errorf("plugin %q exited with error", md.Name)
					}
					return err
				}
				return nil
			},
			// This passes all the flags to the subcommand.
			DisableFlagParsing: true,
		}

		if md.UseTunnel {
			c.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
				// Parse the parent flag, but not the local flags.
				k, _ := manuallyProcessArgs(args)
				if err := c.Parent().ParseFlags(k); err != nil {
					return err
				}
				return setupConnection(cmd, args)
			}
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
	kvargs := []string{"--host", "--kube-context", "--home", "--tiller-namespace"}
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
		case "--host", "--kube-context", "--home":
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

// setupEnv prepares os.Env for plugins. It operates on os.Env because
// the plugin subsystem itself needs access to the environment variables
// created here.
func setupEnv(shortname, base, plugdirs string, home helmpath.Home) {
	// Set extra env vars:
	for key, val := range map[string]string{
		"HELM_PLUGIN_NAME": shortname,
		"HELM_PLUGIN_DIR":  base,
		"HELM_BIN":         os.Args[0],

		// Set vars that may not have been set, and save client the
		// trouble of re-parsing.
		pluginEnvVar: plugdirs,
		homeEnvVar:   home.String(),

		// Set vars that convey common information.
		"HELM_PATH_REPOSITORY":       home.Repository(),
		"HELM_PATH_REPOSITORY_FILE":  home.RepositoryFile(),
		"HELM_PATH_CACHE":            home.Cache(),
		"HELM_PATH_LOCAL_REPOSITORY": home.LocalRepository(),
		"HELM_PATH_STARTER":          home.Starters(),

		"TILLER_HOST":         tillerHost,
		tillerNamespaceEnvVar: tillerNamespace,
	} {
		os.Setenv(key, val)
	}

	if flagDebug {
		os.Setenv("HELM_DEBUG", "1")
	}
}
