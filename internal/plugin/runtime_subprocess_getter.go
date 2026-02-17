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
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func getProtocolCommand(commands []SubprocessProtocolCommand, protocol string) *SubprocessProtocolCommand {
	for _, c := range commands {
		if slices.Contains(c.Protocols, protocol) {
			return &c
		}
	}

	return nil
}

// TODO can we replace a lot of this func with RuntimeSubprocess.invokeWithEnv?
func (r *SubprocessPluginRuntime) runGetter(input *Input) (*Output, error) {
	msg, ok := (input.Message).(schema.InputMessageGetterV1)
	if !ok {
		return nil, fmt.Errorf("expected input type schema.InputMessageGetterV1, got %T", input)
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("helm-plugin-%s-", r.metadata.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	d := getProtocolCommand(r.RuntimeConfig.ProtocolCommands, msg.Protocol)
	if d == nil {
		return nil, fmt.Errorf("no downloader found for protocol %q", msg.Protocol)
	}

	env := ParseEnv(os.Environ())
	maps.Insert(env, maps.All(r.EnvVars))
	maps.Insert(env, maps.All(ParseEnv(input.Env)))
	env["HELM_PLUGIN_NAME"] = r.metadata.Name
	env["HELM_PLUGIN_DIR"] = r.pluginDir
	env["HELM_PLUGIN_USERNAME"] = msg.Options.Username
	env["HELM_PLUGIN_PASSWORD"] = msg.Options.Password
	env["HELM_PLUGIN_PASS_CREDENTIALS_ALL"] = fmt.Sprintf("%t", msg.Options.PassCredentialsAll)

	command, args, err := PrepareCommands(d.PlatformCommand, false, []string{}, env)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare commands for protocol %q: %w", msg.Protocol, err)
	}

	args = append(
		args,
		msg.Options.CertFile,
		msg.Options.KeyFile,
		msg.Options.CAFile,
		msg.Href)

	buf := bytes.Buffer{} // subprocess getters are expected to write content to stdout

	pluginCommand := filepath.Join(r.pluginDir, command)
	cmd := exec.Command(
		pluginCommand,
		args...)
	cmd.Env = FormatEnv(env)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr

	slog.Debug("executing plugin command", slog.String("pluginName", r.metadata.Name), slog.String("command", cmd.String()))
	if err := executeCmd(cmd, r.metadata.Name); err != nil {
		return nil, err
	}

	return &Output{
		Message: schema.OutputMessageGetterV1{
			Data: buf.Bytes(),
		},
	}, nil
}
