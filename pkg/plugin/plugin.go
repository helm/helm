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

package plugin // import "helm.sh/helm/v3/pkg/plugin"

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/cli"
)

const PluginFileName = "plugin.yaml"

// Downloaders represents the plugins capability if it can retrieve
// charts from special sources
type Downloaders struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `json:"protocols"`
	// Command is the executable path with which the plugin performs
	// the actual download for the corresponding Protocols
	Command string `json:"command"`
}

// PlatformCommand represents a command for a particular operating system and architecture
type PlatformCommand struct {
	OperatingSystem string   `json:"os"`
	Architecture    string   `json:"arch"`
	Command         string   `json:"command"`
	Args            []string `json:"args"`
}

// Metadata describes a plugin.
//
// This is the plugin equivalent of a chart.Metadata.
type Metadata struct {
	// Name is the name of the plugin
	Name string `json:"name"`

	// Version is a SemVer 2 version of the plugin.
	Version string `json:"version"`

	// Usage is the single-line usage text shown in help
	Usage string `json:"usage"`

	// Description is a long description shown in places like `helm help`
	Description string `json:"description"`

	// PlatformCommand is the plugin command, with a platform selector and support for args.
	//
	// The command and args will be passed through environment expansion, so env vars can
	// be present in this command. Unless IgnoreFlags is set, this will
	// also merge the flags passed from Helm.
	//
	// Note that the command is not executed in a shell. To do so, we suggest
	// pointing the command to a shell script.
	//
	// The following rules will apply to processing platform commands:
	// - If PlatformCommand is present, it will be used
	// - If both OS and Arch match the current platform, search will stop and the command will be executed
	// - If OS matches and Arch is empty, the command will be executed
	// - If no OS/Arch match is found, the default command will be executed
	// - If no matches are found in platformCommand, Helm will exit with an error
	PlatformCommand []PlatformCommand `json:"platformCommand"`

	// Command is the plugin command, as a single string.
	// Providing a command will result in an error if PlatformCommand is also set.
	//
	// The command will be passed through environment expansion, so env vars can
	// be present in this command. Unless IgnoreFlags is set, this will
	// also merge the flags passed from Helm.
	//
	// Note that command is not executed in a shell. To do so, we suggest
	// pointing the command to a shell script.
	//
	// DEPRECATED: Use PlatformCommand instead. Remove in Helm 4.
	Command string `json:"command"`

	// IgnoreFlags ignores any flags passed in from Helm
	//
	// For example, if the plugin is invoked as `helm --debug myplugin`, if this
	// is false, `--debug` will be appended to `--command`. If this is true,
	// the `--debug` flag will be discarded.
	IgnoreFlags bool `json:"ignoreFlags"`

	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	//
	// The command and args will be passed through environment expansion, so env vars can
	// be present in the command.
	//
	// Note that the command is not executed in a shell. To do so, we suggest
	// pointing the command to a shell script.
	//
	// The following rules will apply to processing platform hooks:
	// - If PlatformHooks is present, it will be used
	// - If both OS and Arch match the current platform, search will stop and the command will be executed
	// - If OS matches and Arch is empty, the command will be executed
	// - If no OS/Arch match is found, the default command will be executed
	// - If no matches are found in platformHooks, Helm will skip the event
	PlatformHooks PlatformHooks `json:"platformHooks"`

	// Hooks are commands that will run on plugin events, as a single string.
	// Providing a hooks will result in an error if PlatformHooks is also set.
	//
	// The command will be passed through environment expansion, so env vars can
	// be present in this command.
	//
	// Note that the command is executed in the sh shell.
	//
	// DEPRECATED: Use PlatformHooks instead. Remove in Helm 4.
	Hooks Hooks

	// Downloaders field is used if the plugin supply downloader mechanism
	// for special protocols.
	Downloaders []Downloaders `json:"downloaders"`

	// UseTunnelDeprecated indicates that this command needs a tunnel.
	// Setting this will cause a number of side effects, such as the
	// automatic setting of HELM_HOST.
	// DEPRECATED and unused, but retained for backwards compatibility with Helm 2 plugins. Remove in Helm 4
	UseTunnelDeprecated bool `json:"useTunnel,omitempty"`
}

// Plugin represents a plugin.
type Plugin struct {
	// Metadata is a parsed representation of a plugin.yaml
	Metadata *Metadata
	// Dir is the string path to the directory that holds the plugin.
	Dir string
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
func PrepareCommands(cmds []PlatformCommand, expandArgs bool, extraArgs []string) (string, []string, error) {
	cmdParts, args := getPlatformCommand(cmds)
	if len(cmdParts) == 0 || cmdParts[0] == "" {
		return "", nil, fmt.Errorf("no plugin command is applicable")
	}

	main := os.ExpandEnv(cmdParts[0])
	baseArgs := []string{}
	if len(cmdParts) > 1 {
		for _, cmdPart := range cmdParts[1:] {
			if expandArgs {
				baseArgs = append(baseArgs, os.ExpandEnv(cmdPart))
			} else {
				baseArgs = append(baseArgs, cmdPart)
			}
		}
	}

	for _, arg := range args {
		if expandArgs {
			baseArgs = append(baseArgs, os.ExpandEnv(arg))
		} else {
			baseArgs = append(baseArgs, arg)
		}
	}

	if len(extraArgs) > 0 {
		baseArgs = append(baseArgs, extraArgs...)
	}

	return main, baseArgs, nil
}

// PrepareCommand gets the correct command and arguments for a plugin.
//
// It merges extraArgs into any arguments supplied in the plugin. It returns the name of the command and an args array.
//
// The result is suitable to pass to exec.Command.
func (p *Plugin) PrepareCommand(extraArgs []string) (string, []string, error) {
	var extraArgsIn []string

	if !p.Metadata.IgnoreFlags {
		extraArgsIn = extraArgs
	}

	cmds := p.Metadata.PlatformCommand
	if len(cmds) == 0 && len(p.Metadata.Command) > 0 {
		cmds = []PlatformCommand{{Command: p.Metadata.Command}}
	}

	return PrepareCommands(cmds, true, extraArgsIn)
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")

// validatePluginData validates a plugin's YAML data.
func validatePluginData(plug *Plugin, filepath string) error {
	// When metadata section missing, initialize with no data
	if plug.Metadata == nil {
		plug.Metadata = &Metadata{}
	}
	if !validPluginName.MatchString(plug.Metadata.Name) {
		return fmt.Errorf("invalid plugin name at %q", filepath)
	}
	plug.Metadata.Usage = sanitizeString(plug.Metadata.Usage)

	if len(plug.Metadata.PlatformCommand) > 0 && len(plug.Metadata.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set in %q", filepath)
	}

	if len(plug.Metadata.PlatformHooks) > 0 && len(plug.Metadata.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set in %q", filepath)
	}

	// We could also validate SemVer, executable, and other fields should we so choose.
	return nil
}

// sanitizeString normalize spaces and removes non-printable characters.
func sanitizeString(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, str)
}

func detectDuplicates(plugs []*Plugin) error {
	names := map[string]string{}

	for _, plug := range plugs {
		if oldpath, ok := names[plug.Metadata.Name]; ok {
			return fmt.Errorf(
				"two plugins claim the name %q at %q and %q",
				plug.Metadata.Name,
				oldpath,
				plug.Dir,
			)
		}
		names[plug.Metadata.Name] = plug.Dir
	}

	return nil
}

// LoadDir loads a plugin from the given directory.
func LoadDir(dirname string) (*Plugin, error) {
	pluginfile := filepath.Join(dirname, PluginFileName)
	data, err := os.ReadFile(pluginfile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read plugin at %q", pluginfile)
	}

	plug := &Plugin{Dir: dirname}
	if err := yaml.UnmarshalStrict(data, &plug.Metadata); err != nil {
		return nil, errors.Wrapf(err, "failed to load plugin at %q", pluginfile)
	}
	return plug, validatePluginData(plug, pluginfile)
}

// LoadAll loads all plugins found beneath the base directory.
//
// This scans only one directory level.
func LoadAll(basedir string) ([]*Plugin, error) {
	plugins := []*Plugin{}
	// We want basedir/*/plugin.yaml
	scanpath := filepath.Join(basedir, "*", PluginFileName)
	matches, err := filepath.Glob(scanpath)
	if err != nil {
		return plugins, errors.Wrapf(err, "failed to find plugins in %q", scanpath)
	}

	if matches == nil {
		return plugins, nil
	}

	for _, yaml := range matches {
		dir := filepath.Dir(yaml)
		p, err := LoadDir(dir)
		if err != nil {
			return plugins, err
		}
		plugins = append(plugins, p)
	}
	return plugins, detectDuplicates(plugins)
}

// FindPlugins returns a list of YAML files that describe plugins.
func FindPlugins(plugdirs string) ([]*Plugin, error) {
	found := []*Plugin{}
	// Let's get all UNIXy and allow path separators
	for _, p := range filepath.SplitList(plugdirs) {
		matches, err := LoadAll(p)
		if err != nil {
			return matches, err
		}
		found = append(found, matches...)
	}
	return found, nil
}

// SetupPluginEnv prepares os.Env for plugins. It operates on os.Env because
// the plugin subsystem itself needs access to the environment variables
// created here.
func SetupPluginEnv(settings *cli.EnvSettings, name, base string) {
	env := settings.EnvVars()
	env["HELM_PLUGIN_NAME"] = name
	env["HELM_PLUGIN_DIR"] = base
	for key, val := range env {
		os.Setenv(key, val)
	}
}
