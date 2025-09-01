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
	"os"
	"os/exec"
	"syscall"

	"helm.sh/helm/v4/internal/plugin/schema"
	"helm.sh/helm/v4/pkg/cli"
)

// SubprocessProtocolCommand maps a given protocol to the getter command used to retrieve artifacts for that protcol
type SubprocessProtocolCommand struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `yaml:"protocols"`
	// Command is the executable path with which the plugin performs
	// the actual download for the corresponding Protocols
	Command string `yaml:"command"`
}

// RuntimeConfigSubprocess implements RuntimeConfig for RuntimeSubprocess
type RuntimeConfigSubprocess struct {
	// PlatformCommand is a list containing a plugin command, with a platform selector and support for args.
	PlatformCommands []PlatformCommand `yaml:"platformCommand"`
	// Command is the plugin command, as a single string.
	// DEPRECATED: Use PlatformCommand instead. Remove in Helm 4.
	Command string `yaml:"command"`
	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `yaml:"platformHooks"`
	// Hooks are commands that will run on plugin events, as a single string.
	// DEPRECATED: Use PlatformHooks instead. Remove in Helm 4.
	Hooks Hooks `yaml:"hooks"`
	// ProtocolCommands field is used if the plugin supply downloader mechanism
	// for special protocols.
	// (This is a compatibility hangover from the old plugin downloader mechanism, which was extended to support multiple
	// protocols in a given plugin)
	ProtocolCommands []SubprocessProtocolCommand `yaml:"protocolCommands,omitempty"`
}

var _ RuntimeConfig = (*RuntimeConfigSubprocess)(nil)

func (r *RuntimeConfigSubprocess) GetType() string { return "subprocess" }

func (r *RuntimeConfigSubprocess) Validate() error {
	if len(r.PlatformCommands) > 0 && len(r.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set")
	}
	if len(r.PlatformHooks) > 0 && len(r.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set")
	}
	return nil
}

type RuntimeSubprocess struct{}

var _ Runtime = (*RuntimeSubprocess)(nil)

// CreatePlugin implementation for Runtime
func (r *RuntimeSubprocess) CreatePlugin(pluginDir string, metadata *Metadata) (Plugin, error) {
	return &SubprocessPluginRuntime{
		metadata:      *metadata,
		pluginDir:     pluginDir,
		RuntimeConfig: *(metadata.RuntimeConfig.(*RuntimeConfigSubprocess)),
	}, nil
}

// SubprocessPluginRuntime implements the Plugin interface for subprocess execution
type SubprocessPluginRuntime struct {
	metadata      Metadata
	pluginDir     string
	RuntimeConfig RuntimeConfigSubprocess
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
	prog := exec.Command(mainCmdExp, argv...)
	prog.Env = env
	prog.Stdin = stdin
	prog.Stdout = stdout
	prog.Stderr = stderr

	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			status := eerr.Sys().(syscall.WaitStatus)
			return &InvokeExecError{
				Err:  fmt.Errorf("plugin %q exited with error", r.metadata.Name),
				Code: status.ExitStatus(),
			}
		}
	}
	return nil
}

func (r *SubprocessPluginRuntime) InvokeHook(event string) error {
	// Get hook commands for the event
	var cmds []PlatformCommand
	expandArgs := true

	cmds = r.RuntimeConfig.PlatformHooks[event]
	if len(cmds) == 0 && len(r.RuntimeConfig.Hooks) > 0 {
		cmd := r.RuntimeConfig.Hooks[event]
		if len(cmd) > 0 {
			cmds = []PlatformCommand{{Command: "sh", Args: []string{"-c", cmd}}}
			expandArgs = false
		}
	}

	// If no hook commands are defined, just return successfully
	if len(cmds) == 0 {
		return nil
	}

	main, argv, err := PrepareCommands(cmds, expandArgs, []string{})
	if err != nil {
		return err
	}

	prog := exec.Command(main, argv...)
	prog.Stdout, prog.Stderr = os.Stdout, os.Stderr

	if err := prog.Run(); err != nil {
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
			os.Stderr.Write(eerr.Stderr)
			return &InvokeExecError{
				Err:  fmt.Errorf("plugin %q exited with error", pluginName),
				Code: eerr.ExitCode(),
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

	cmds := r.RuntimeConfig.PlatformCommands
	if len(cmds) == 0 && len(r.RuntimeConfig.Command) > 0 {
		cmds = []PlatformCommand{{Command: r.RuntimeConfig.Command}}
	}

	command, args, err := PrepareCommands(cmds, true, extraArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare plugin command: %w", err)
	}

	err2 := r.InvokeWithEnv(command, args, input.Env, input.Stdin, input.Stdout, input.Stderr)
	if err2 != nil {
		return nil, err2
	}

	return &Output{
		Message: schema.OutputMessageCLIV1{},
	}, nil
}

func (r *SubprocessPluginRuntime) runPostrenderer(input *Input) (*Output, error) {
	if _, ok := input.Message.(schema.InputMessagePostRendererV1); !ok {
		return nil, fmt.Errorf("plugin %q input message does not implement InputMessagePostRendererV1", r.metadata.Name)
	}

	msg := input.Message.(schema.InputMessagePostRendererV1)
	extraArgs := msg.ExtraArgs
	settings := msg.Settings

	// Setup plugin environment
	SetupPluginEnv(settings, r.metadata.Name, r.pluginDir)

	cmds := r.RuntimeConfig.PlatformCommands
	if len(cmds) == 0 && len(r.RuntimeConfig.Command) > 0 {
		cmds = []PlatformCommand{{Command: r.RuntimeConfig.Command}}
	}

	command, args, err := PrepareCommands(cmds, true, extraArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare plugin command: %w", err)
	}

	// TODO de-duplicate code here by calling RuntimeSubprocess.invokeWithEnv()
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

	//cmd.Env = pluginExec.env
	cmd.Stdout = postRendered
	cmd.Stderr = stderr

	if err := executeCmd(cmd, r.metadata.Name); err != nil {
		slog.Info("plugin execution failed", slog.String("stderr", stderr.String()))
		return nil, err
	}

	return &Output{
		Message: &schema.OutputMessagePostRendererV1{
			Manifests: postRendered,
		},
	}, nil
}

// SetupPluginEnv prepares os.Env for plugins. It operates on os.Env because
// the plugin subsystem itself needs access to the environment variables
// created here.
func SetupPluginEnv(settings *cli.EnvSettings, name, base string) { // TODO: remove
	env := settings.EnvVars()
	env["HELM_PLUGIN_NAME"] = name
	env["HELM_PLUGIN_DIR"] = base
	for key, val := range env {
		os.Setenv(key, val)
	}
}
