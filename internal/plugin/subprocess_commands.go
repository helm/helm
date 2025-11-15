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

package plugin

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// PlatformCommand represents a command for a particular operating system and architecture
type PlatformCommand struct {
	OperatingSystem string   `yaml:"os"`
	Architecture    string   `yaml:"arch"`
	Command         string   `yaml:"command"`
	Args            []string `yaml:"args"`
}

// Returns command and args strings based on the following rules in priority order:
// - From the PlatformCommand where OS and Arch match the current platform
// - From the PlatformCommand where OS matches the current platform and Arch is empty/unspecified
// - From the PlatformCommand where OS is empty/unspecified and Arch matches the current platform
// - From the PlatformCommand where OS and Arch are both empty/unspecified
// - Return nil, nil
func getPlatformCommand(cmds []PlatformCommand) ([]string, []string) {
	var command, args []string
	found := false
	foundOs := false

	eq := strings.EqualFold
	for _, c := range cmds {
		if eq(c.OperatingSystem, runtime.GOOS) && eq(c.Architecture, runtime.GOARCH) {
			// Return early for an exact match
			return strings.Split(c.Command, " "), c.Args
		}

		if (len(c.OperatingSystem) > 0 && !eq(c.OperatingSystem, runtime.GOOS)) || len(c.Architecture) > 0 {
			// Skip if OS is not empty and doesn't match or if arch is set as a set arch requires an OS match
			continue
		}

		if !foundOs && len(c.OperatingSystem) > 0 && eq(c.OperatingSystem, runtime.GOOS) {
			// First OS match with empty arch, can only be overridden by a direct match
			command = strings.Split(c.Command, " ")
			args = c.Args
			found = true
			foundOs = true
		} else if !found {
			// First empty match, can be overridden by a direct match or an OS match
			command = strings.Split(c.Command, " ")
			args = c.Args
			found = true
		}
	}

	return command, args
}

// PrepareCommands takes a []Plugin.PlatformCommand
// and prepares the command and arguments for execution.
//
// It merges extraArgs into any arguments supplied in the plugin. It
// returns the main command and an args array.
//
// The result is suitable to pass to exec.Command.
func PrepareCommands(cmds []PlatformCommand, expandArgs bool, extraArgs []string, env map[string]string) (string, []string, error) {
	cmdParts, args := getPlatformCommand(cmds)
	if len(cmdParts) == 0 || cmdParts[0] == "" {
		return "", nil, fmt.Errorf("no plugin command is applicable")
	}
	envMappingFunc := func(key string) string {
		return env[key]
	}

	main := os.Expand(cmdParts[0], envMappingFunc)
	baseArgs := []string{}
	if len(cmdParts) > 1 {
		for _, cmdPart := range cmdParts[1:] {
			if expandArgs {
				baseArgs = append(baseArgs, os.Expand(cmdPart, envMappingFunc))
			} else {
				baseArgs = append(baseArgs, cmdPart)
			}
		}
	}

	for _, arg := range args {
		if expandArgs {
			baseArgs = append(baseArgs, os.Expand(arg, envMappingFunc))
		} else {
			baseArgs = append(baseArgs, arg)
		}
	}

	if len(extraArgs) > 0 {
		baseArgs = append(baseArgs, extraArgs...)
	}

	return main, baseArgs, nil
}
