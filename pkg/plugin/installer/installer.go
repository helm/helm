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
        "io/ioutil"
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
        OperatingSystem string `json:"os"`
        Architecture    string `json:"arch"`
        Command         string `json:"command"`
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

        // Command is the command, as a single string.
        //
        // The command will be passed through environment expansion, so env vars can
        // be present in this command. Unless IgnoreFlags is set, this will
        // also merge the flags passed from Helm.
        //
        // Note that command is not executed in a shell. To do so, we suggest
        // pointing the command to a shell script.
        //
        // The following rules will apply to processing commands:
        // - If platformCommand is present, it will be searched first
        // - If both OS and Arch match the current platform, search will stop and the command will be executed
        // - If OS matches and there is no more specific match, the command will be executed
        // - If no OS/Arch match is found, the default command will be executed
        // - If no command is present and no matches are found in platformCommand, Helm will exit with an error
        PlatformCommand []PlatformCommand `json:"platformCommand"`
        Command         string            `json:"command"`

        // IgnoreFlags ignores any flags passed in from Helm
        //
        // For example, if the plugin is invoked as `helm --debug myplugin`, if this
        // is false, `--debug` will be appended to `--command`. If this is true,
        // the `--debug` flag will be discarded.
        IgnoreFlags bool `json:"ignoreFlags"`

        // Hooks are commands that will run on events.
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

// The following rules will apply to processing the Plugin.PlatformCommand.Command:
// - If both OS and Arch match the current platform, search will stop and the command will be prepared for execution
// - If OS matches and there is no more specific match, the command will be prepared for execution
// - If no OS/Arch match is found, return nil
func getPlatformCommand(cmds []PlatformCommand) []string {
        var command []string
        eq := strings.EqualFold
        for _, c := range cmds {
                if eq(c.OperatingSystem, runtime.GOOS) {
                        command = strings.Split(os.ExpandEnv(c.Command), " ")
                }
                if eq(c.OperatingSystem, runtime.GOOS) && eq(c.Architecture, runtime.GOARCH) {
                        return strings.Split(os.ExpandEnv(c.Command), " ")
                }
        }
        return command
}

// PrepareCommand takes a Plugin.PlatformCommand.Command, a Plugin.Command and will applying the following processing:
// - If platformCommand is present, it will be searched first
// - If both OS and Arch match the current platform, search will stop and the command will be prepared for execution
// - If OS matches and there is no more specific match, the command will be prepared for execution
// - If no OS/Arch match is found, the default command will be prepared for execution
// - If no command is present and no matches are found in platformCommand, will exit with an error
//
// It merges extraArgs into any arguments supplied in the plugin. It
// returns the name of the command and an args array.
//
// The result is suitable to pass to exec.Command.
func (p *Plugin) PrepareCommand(extraArgs []string) (string, []string, error) {
        var parts []string
        platCmdLen := len(p.Metadata.PlatformCommand)
        if platCmdLen > 0 {
                parts = getPlatformCommand(p.Metadata.PlatformCommand)
        }
        if platCmdLen == 0 || parts == nil {
                parts = strings.Split(os.ExpandEnv(p.Metadata.Command), " ")
        }
        if len(parts) == 0 || parts[0] == "" {
                return "", nil, fmt.Errorf("no plugin command is applicable")
        }

        main := parts[0]
        baseArgs := []string{}
        if len(parts) > 1 {
                baseArgs = parts[1:]
        }
        if !p.Metadata.IgnoreFlags {
                baseArgs = append(baseArgs, extraArgs...)
        }
        return main, baseArgs, nil
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")

// validatePluginData validates a plugin's YAML data.
func validatePluginData(plug *Plugin, filepath string) error {
        if !validPluginName.MatchString(plug.Metadata.Name) {
                return fmt.Errorf("invalid plugin name at %q", filepath)
        }
        plug.Metadata.Usage = sanitizeString(plug.Metadata.Usage)

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
        data, err := ioutil.ReadFile(pluginfile)
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
PS C:\workspace\go\helm\helm> gofmt -s .\pkg\plugin\installer\installer.go
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

package installer

import (
        "fmt"
        "log"
        "net/http"
        "os"
        "path/filepath"
        "strings"

        "github.com/pkg/errors"

        "helm.sh/helm/v3/pkg/plugin"
)

// ErrMissingMetadata indicates that plugin.yaml is missing.
var ErrMissingMetadata = errors.New("plugin metadata (plugin.yaml) missing")

// Debug enables verbose output.
var Debug bool

// Installer provides an interface for installing helm client plugins.
type Installer interface {
        // Install adds a plugin.
        Install() error
        // Path is the directory of the installed plugin.
        Path() string
        // Update updates a plugin.
        Update() error
}

// Install installs a plugin.
func Install(i Installer) error {
        if err := os.MkdirAll(filepath.Dir(i.Path()), 0755); err != nil {
                return err
        }
        if _, pathErr := os.Stat(i.Path()); !os.IsNotExist(pathErr) {
                return errors.New("plugin already exists")
        }
        return i.Install()
}

// Update updates a plugin.
func Update(i Installer) error {
        if _, pathErr := os.Stat(i.Path()); os.IsNotExist(pathErr) {
                return errors.New("plugin does not exist")
        }
        return i.Update()
}

// NewForSource determines the correct Installer for the given source.
func NewForSource(source, version string) (Installer, error) {
        // Check if source is a local directory
        if isLocalReference(source) {
                return NewLocalInstaller(source)
        } else if isRemoteHTTPArchive(source) {
                return NewHTTPInstaller(source)
        }
        return NewVCSInstaller(source, version)
}

// FindSource determines the correct Installer for the given source.
func FindSource(location string) (Installer, error) {
        installer, err := existingVCSRepo(location)
        if err != nil && err.Error() == "Cannot detect VCS" {
                return installer, errors.New("cannot get information about plugin source")
        }
        return installer, err
}

// isLocalReference checks if the source exists on the filesystem.
func isLocalReference(source string) bool {
        _, err := os.Stat(source)
        return err == nil
}

// isRemoteHTTPArchive checks if the source is a http/https url and is an archive
//
// It works by checking whether the source looks like a URL and, if it does, running a
// HEAD operation to see if the remote resource is a file that we understand.
func isRemoteHTTPArchive(source string) bool {
        if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
                res, err := http.Head(source)
                if err != nil {
                        // If we get an error at the network layer, we can't install it. So
                        // we return false.
                        return false
                }

                // Next, we look for the content type or content disposition headers to see
                // if they have matching extractors.
                contentType := res.Header.Get("content-type")
                foundSuffix, ok := mediaTypeToExtension(contentType)
                if !ok {
                        // Media type not recognized
                        return false
                }

                for suffix := range Extractors {
                        if strings.HasSuffix(foundSuffix, suffix) {
                                return true
                        }
                }
        }
        return false
}

// isPlugin checks if the directory contains a plugin.yaml file.
func isPlugin(dirname string) bool {
        _, err := os.Stat(filepath.Join(dirname, plugin.PluginFileName))
        return err == nil
}

var logger = log.New(os.Stderr, "[debug] ", log.Lshortfile)

func debug(format string, args ...interface{}) {
        if Debug {
                logger.Output(2, fmt.Sprintf(format, args...))
        }
}