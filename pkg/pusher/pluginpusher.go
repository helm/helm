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

package pusher

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/plugin"
)

// collectPlugins scans for pusher plugins.
// This will load plugins according to the cli.
func collectPlugins(settings *cli.EnvSettings) (Providers, error) {
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err != nil {
		return nil, err
	}
	var result Providers
	for _, plugin := range plugins {
		for _, uploader := range plugin.Metadata.Uploaders {
			result = append(result, Provider{
				Schemes: uploader.Protocols,
				New: NewPluginPusher(
					uploader.Command,
					settings,
					plugin.Metadata.Name,
					plugin.Dir,
				),
			})
		}
	}
	return result, nil
}

// pluginPusher is a generic type to invoke custom uploaders,
// implemented in plugins.
type pluginPusher struct {
	command  string
	settings *cli.EnvSettings
	name     string
	base     string
	opts     options
}

// Push runs uploader plugin command
func (p *pluginPusher) Push(chartRef, href string, options ...Option) error {
	for _, opt := range options {
		opt(&p.opts)
	}
	commands := strings.Split(p.command, " ")
	argv := append(commands[1:], p.opts.certFile, p.opts.keyFile, p.opts.caFile, href, chartRef)
	prog := exec.Command(filepath.Join(p.base, commands[0]), argv...)
	plugin.SetupPluginEnv(p.settings, p.name, p.base)
	prog.Env = os.Environ()
	buf := bytes.NewBuffer(nil)
	prog.Stdout = buf
	prog.Stderr = os.Stderr
	if err := prog.Run(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(eerr.Stderr)
			return errors.Errorf("plugin %q exited with error", p.command)
		}
		return err
	}
	return nil
}

// NewPluginPusher constructs a valid plugin pusher
func NewPluginPusher(command string, settings *cli.EnvSettings, name, base string) Constructor {
	return func(options ...Option) (Pusher, error) {
		result := &pluginPusher{
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
