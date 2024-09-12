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

package getter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/plugin"
)

// collectPlugins scans for getter plugins.
// This will load plugins according to the cli.
func collectPlugins(settings *cli.EnvSettings) (Providers, error) {
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err != nil {
		return nil, err
	}
	var result Providers
	for _, plugin := range plugins {
		for _, downloader := range plugin.Metadata.Downloaders {
			result = append(result, Provider{
				Schemes: downloader.Protocols,
				New: NewPluginGetter(
					downloader.Command,
					settings,
					plugin.Metadata.Name,
					plugin.Dir,
				),
			})
		}
	}
	return result, nil
}

// pluginGetter is a generic type to invoke custom downloaders,
// implemented in plugins.
type pluginGetter struct {
	command  string
	settings *cli.EnvSettings
	name     string
	base     string
	opts     options
}

func (p *pluginGetter) setupOptionsEnv(env []string) []string {
	env = append(env, fmt.Sprintf("HELM_PLUGIN_USERNAME=%s", p.opts.username))
	env = append(env, fmt.Sprintf("HELM_PLUGIN_PASSWORD=%s", p.opts.password))
	env = append(env, fmt.Sprintf("HELM_PLUGIN_PASS_CREDENTIALS_ALL=%t", p.opts.passCredentialsAll))
	return env
}

// Get runs downloader plugin command
func (p *pluginGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&p.opts)
	}
	commands := strings.Split(p.command, " ")
	argv := append(commands[1:], p.opts.certFile, p.opts.keyFile, p.opts.caFile, href)
	prog := exec.Command(filepath.Join(p.base, commands[0]), argv...)
	plugin.SetupPluginEnv(p.settings, p.name, p.base)
	prog.Env = p.setupOptionsEnv(os.Environ())
	buf := bytes.NewBuffer(nil)
	prog.Stdout = buf
	prog.Stderr = os.Stderr
	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return nil, errors.Errorf("plugin %q exited with error", p.command)
		}
		return nil, err
	}
	return buf, nil
}

// NewPluginGetter constructs a valid plugin getter
func NewPluginGetter(command string, settings *cli.EnvSettings, name, base string) Constructor {
	return func(options ...Option) (Getter, error) {
		result := &pluginGetter{
			command:  command,
			settings: settings,
			name:     name,
			base:     base,
		}
		for _, opt := range options {
			opt(&result.opts)
		}
		return result, nil
	}
}
