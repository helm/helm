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
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"slices"

	"helm.sh/helm/v4/internal/plugin/schema"
)

// SubprocessProtocolCommand maps a given protocol to the getter command used to retrieve artifacts for that protocol
type SubprocessProtocolCommand struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `yaml:"protocols"`
	// PlatformCommand is the platform based command which the plugin performs
	// to download for the corresponding getter Protocols.
	PlatformCommand []PlatformCommand `yaml:"platformCommand"`
}

// RuntimeConfigSubprocess implements RuntimeConfig for RuntimeSubprocess
type RuntimeConfigSubprocess struct {
	// PlatformCommand is a list containing a plugin command, with a platform selector and support for args.
	PlatformCommand []PlatformCommand `yaml:"platformCommand"`
	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `yaml:"platformHooks"`
	// ProtocolCommands allows the plugin to specify protocol specific commands
	//
	// Obsolete/deprecated: This is a compatibility hangover from the old plugin downloader mechanism, which was extended
	// to support multiple protocols in a given plugin. The command supplied in PlatformCommand should implement protocol
	// specific logic by inspecting the download URL
	ProtocolCommands []SubprocessProtocolCommand `yaml:"protocolCommands,omitempty"`

	expandHookArgs bool
}

var _ RuntimeConfig = (*RuntimeConfigSubprocess)(nil)

func (r *RuntimeConfigSubprocess) GetType() string { return "subprocess" }

func (r *RuntimeConfigSubprocess) Validate() error {
	return nil
}

type RuntimeSubprocess struct {
	EnvVars map[string]string
}

var _ Runtime = (*RuntimeSubprocess)(nil)

// CreatePlugin implementation for Runtime
func (r *RuntimeSubprocess) CreatePlugin(pluginDir string, metadata *Metadata) (Plugin, error) {
	return &SubprocessPluginRuntime{
		metadata:      *metadata,
		pluginDir:     pluginDir,
		RuntimeConfig: *(metadata.RuntimeConfig.(*RuntimeConfigSubprocess)),
		EnvVars:       maps.Clone(r.EnvVars),
	}, nil
}

// SubprocessPluginRuntime implements the Plugin interface for subprocess execution
type SubprocessPluginRuntime struct {
	metadata      Metadata
	pluginDir     string
	RuntimeConfig RuntimeConfigSubprocess
	EnvVars       map[string]string
}

var _ Plugin = (*SubprocessPluginRuntime)(nil)

func (r *SubprocessPluginRuntime) Dir() string {
	return r.pluginDir
}

func (r *SubprocessPluginRuntime) Metadata() Metadata {
	return r.metadata
}

func (r *SubprocessPluginRuntime) Invoke(_ context.Context, input *Input) (*Output, error) {
	switch input.Message.(type) {
	case schema.InputMessageCLIV1:
		return r.runCLI(input)
	case schema.InputMessageGetterV1:
		return r.runGetter(input)
	case schema.InputMessagePostRendererV1:
		return r.runPostrenderer(input)
	default:
		return nil, fmt.Errorf("unsupported subprocess plugin type %q", r.metadata.Type)
	}
}

// InvokeWithEnv executes a plugin command with custom environment and I/O streams
// This method allows execution with different command/args than the plugin's default
func (r *SubprocessPluginRuntime) InvokeWithEnv(main string, argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	mainCmdExp := os.ExpandEnv(main)
	cmd := exec.Command(mainCmdExp, argv...)
	cmd.Env = slices.Clone(os.Environ())
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("HELM_PLUGIN_NAME=%s", r.metadata.Name),
		fmt.Sprintf("HELM_PLUGIN_DIR=%s", r.pluginDir))
	cmd.Env = append(cmd.Env, env...)

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := executeCmd(cmd, r.metadata.Name); err != nil {
		return err
	}

	return nil
}

func (r *SubprocessPluginRuntime) InvokeHook(event string) error {
	cmds := r.RuntimeConfig.PlatformHooks[event]

	if len(cmds) == 0 {
		return nil
	}

	env := ParseEnv(os.Environ())
	maps.Insert(env, maps.All(r.EnvVars))
	env["HELM_PLUGIN_NAME"] = r.metadata.Name
	env["HELM_PLUGIN_DIR"] = r.pluginDir

	main, argv, err := PrepareCommands(cmds, r.RuntimeConfig.expandHookArgs, []string{}, env)
	if err != nil {
		return err
	}

	cmd := exec.Command(main, argv...)
	cmd.Env = FormatEnv(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	slog.Debug("executing plugin hook command", slog.String("pluginName", r.metadata.Name), slog.String("command", cmd.String()))
	if err := cmd.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return fmt.Errorf("plugin %s hook for %q exited with error", event, r.metadata.Name)
		}
		return err
	}
	return nil
}

// TODO decide the best way to handle this code
// right now we implement status and error return in 3 slightly different ways in this file
// then replace the other three with a call to this func
func executeCmd(prog *exec.Cmd, pluginName string) error {
	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			slog.Debug(
				"plugin execution failed",
				slog.String("pluginName", pluginName),
				slog.String("error", err.Error()),
				slog.Int("exitCode", eerr.ExitCode()),
				slog.String("stderr", string(bytes.TrimSpace(eerr.Stderr))))
			return &InvokeExecError{
				Err:      fmt.Errorf("plugin %q exited with error", pluginName),
				ExitCode: eerr.ExitCode(),
			}
		}

		return err
	}

	return nil
}

func (r *SubprocessPluginRuntime) runCLI(input *Input) (*Output, error) {
	if _, ok := input.Message.(schema.InputMessageCLIV1); !ok {
		return nil, fmt.Errorf("plugin %q input message does not implement InputMessageCLIV1", r.metadata.Name)
	}

	extraArgs := input.Message.(schema.InputMessageCLIV1).ExtraArgs

	cmds := r.RuntimeConfig.PlatformCommand

	env := ParseEnv(os.Environ())
	maps.Insert(env, maps.All(r.EnvVars))
	maps.Insert(env, maps.All(ParseEnv(input.Env)))
	env["HELM_PLUGIN_NAME"] = r.metadata.Name
	env["HELM_PLUGIN_DIR"] = r.pluginDir

	command, args, err := PrepareCommands(cmds, true, extraArgs, env)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare plugin command: %w", err)
	}

	cmd := exec.Command(command, args...)
	cmd.Env = FormatEnv(env)

	cmd.Stdin = input.Stdin
	cmd.Stdout = input.Stdout
	cmd.Stderr = input.Stderr

	slog.Debug("executing plugin command", slog.String("pluginName", r.metadata.Name), slog.String("command", cmd.String()))
	if err := executeCmd(cmd, r.metadata.Name); err != nil {
		return nil, err
	}

	return &Output{
		Message: schema.OutputMessageCLIV1{},
	}, nil
}

func (r *SubprocessPluginRuntime) runPostrenderer(input *Input) (*Output, error) {
	if _, ok := input.Message.(schema.InputMessagePostRendererV1); !ok {
		return nil, fmt.Errorf("plugin %q input message does not implement InputMessagePostRendererV1", r.metadata.Name)
	}

	env := ParseEnv(os.Environ())
	maps.Insert(env, maps.All(r.EnvVars))
	maps.Insert(env, maps.All(ParseEnv(input.Env)))
	env["HELM_PLUGIN_NAME"] = r.metadata.Name
	env["HELM_PLUGIN_DIR"] = r.pluginDir

	msg := input.Message.(schema.InputMessagePostRendererV1)
	cmds := r.RuntimeConfig.PlatformCommand
	command, args, err := PrepareCommands(cmds, true, msg.ExtraArgs, env)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare plugin command: %w", err)
	}

	cmd := exec.Command(
		command,
		args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, msg.Manifests)
	}()

	postRendered := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Env = FormatEnv(env)
	cmd.Stdout = postRendered
	cmd.Stderr = stderr

	slog.Debug("executing plugin command", slog.String("pluginName", r.metadata.Name), slog.String("command", cmd.String()))
	if err := executeCmd(cmd, r.metadata.Name); err != nil {
		return nil, err
	}

	return &Output{
		Message: schema.OutputMessagePostRendererV1{
			Manifests: postRendered,
		},
	}, nil
}
