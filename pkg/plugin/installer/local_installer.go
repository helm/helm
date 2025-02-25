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

package installer // import "helm.sh/helm/v4/pkg/plugin/installer"

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// ErrPluginNotAFolder indicates that the plugin path is not a folder.
var ErrPluginNotAFolder = errors.New("expected plugin to be a folder")

// LocalInstaller installs plugins from the filesystem.
type LocalInstaller struct {
	base
}

// NewLocalInstaller creates a new LocalInstaller.
func NewLocalInstaller(source string) (*LocalInstaller, error) {
	src, err := filepath.Abs(source)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get absolute path to plugin")
	}
	i := &LocalInstaller{
		base: newBase(src),
	}
	return i, nil
}

// Install creates a symlink to the plugin directory.
//
// Implements Installer.
func (i *LocalInstaller) Install() error {
	stat, err := os.Stat(i.Source)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return ErrPluginNotAFolder
	}

	if !isPlugin(i.Source) {
		return ErrMissingMetadata
	}
	debug("symlinking %s to %s", i.Source, i.Path())
	return os.Symlink(i.Source, i.Path())
}

// Update updates a local repository
func (i *LocalInstaller) Update() error {
	debug("local repository is auto-updated")
	return nil
}
