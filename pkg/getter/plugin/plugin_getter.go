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

package plugingetter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/plugin"
)

// PluginGetter is a generic type to invoke custom downloaders,
// implemented in plugins.
type PluginGetter struct {
	command                   string
	certFile, keyFile, cAFile string
	settings                  environment.EnvSettings
	name                      string
	base                      string
}

// Get runs downloader plugin command
func (p *PluginGetter) Get(href string) (*bytes.Buffer, error) {
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
			return nil, fmt.Errorf("plugin %q exited with error", p.command)
		}
		return nil, err
	}
	return buf, nil
}

// ConstructNew constructs a valid plugin getter
func ConstructNew(command string,
	settings environment.EnvSettings,
	name string,
	base string) getter.Constructor {
	return func(URL, CertFile, KeyFile, CAFile string) (getter.Getter, error) {
		result := &PluginGetter{
			command:  command,
			certFile: CertFile,
			keyFile:  KeyFile,
			cAFile:   CAFile,
			settings: settings,
			name:     name,
			base:     base,
		}
		return result, nil
	}
}
