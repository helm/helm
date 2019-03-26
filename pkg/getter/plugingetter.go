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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/plugin"
)

// PluginDownloaderPrefix is the command name prefix for all helm downloader plugins.
const PluginDownloaderPrefix = "downloader-"

// collectPlugins scans for getter plugins.
// This will load plugins according to the cli.
func collectPlugins(settings cli.EnvSettings) (Providers, error) {
	plugins, err := plugin.FindAll(os.Getenv("PATH"))
	if err != nil {
		return nil, err
	}
	var result Providers
	for _, plugin := range plugins {
		if strings.HasPrefix(plugin.Name, PluginDownloaderPrefix) {
			scheme := strings.TrimPrefix(plugin.Name, PluginDownloaderPrefix)
			result = append(result, Provider{
				Schemes: []string{scheme},
				New:     newPluginGetter(plugin, settings),
			})
		}
	}
	return result, nil
}

// pluginGetter is a generic type to invoke custom downloaders,
// implemented in plugins.
type pluginGetter struct {
	command                   string
	certFile, keyFile, cAFile string
	settings                  cli.EnvSettings
	name                      string
	base                      string
}

// Get runs downloader plugin command
func (p *pluginGetter) Get(href string) (*bytes.Buffer, error) {
	argv := []string{p.certFile, p.keyFile, p.cAFile, href}
	prog := exec.Command(filepath.Join(p.base, p.command), argv...)
	plugin.SetupPluginEnv(p.settings, p.name, p.base)
	prog.Env = os.Environ()
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

// newPluginGetter constructs a valid plugin getter
func newPluginGetter(plugin *plugin.Plugin, settings cli.EnvSettings) Constructor {
	return func(URL, CertFile, KeyFile, CAFile string) (Getter, error) {
		result := &pluginGetter{
			command:  plugin.Name,
			certFile: CertFile,
			keyFile:  KeyFile,
			cAFile:   CAFile,
			settings: settings,
			name:     plugin.Name,
			base:     plugin.Dir,
		}
		return result, nil
	}
}
