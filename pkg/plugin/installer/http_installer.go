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

package installer // import "k8s.io/helm/pkg/plugin/installer"

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/plugin/cache"
)

// HTTPInstaller installs plugins from an archive served by a web server.
type HTTPInstaller struct {
	CacheDir   string
	PluginName string
	base
	extractor Extractor
	getter    getter.Getter
}

// NewHTTPInstaller creates a new HttpInstaller.
func NewHTTPInstaller(source string, home helmpath.Home) (*HTTPInstaller, error) {

	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	extractor, err := NewExtractor(source)
	if err != nil {
		return nil, err
	}

	getConstructor, err := getter.ByScheme("http", environment.EnvSettings{})
	if err != nil {
		return nil, err
	}

	get, err := getConstructor.New(source, "", "", "")
	if err != nil {
		return nil, err
	}

	i := &HTTPInstaller{
		CacheDir:   home.Path("cache", "plugins", key),
		PluginName: stripPluginName(filepath.Base(source)),
		base:       newBase(source, home),
		extractor:  extractor,
		getter:     get,
	}
	return i, nil
}

// Install downloads and extracts the tarball into the cache directory and creates a symlink to the plugin directory in $HELM_HOME.
//
// Implements Installer.
func (i *HTTPInstaller) Install() error {

	pluginData, err := i.getter.Get(i.Source)
	if err != nil {
		return err
	}

	pluginDir, err := i.extractor.Extract(pluginData, i.CacheDir)
	if err != nil {
		return err
	}
	i.CacheDir = pluginDir // plugin.yaml could be in a sub-folder

	if !isPlugin(i.CacheDir) {
		return ErrMissingMetadata
	}

	src, err := filepath.Abs(i.CacheDir)
	if err != nil {
		return err
	}

	return i.link(src)
}

// Update updates a local repository
// Not implemented for now since tarball most likely will be packaged by version
func (i *HTTPInstaller) Update() error {
	return fmt.Errorf("method Update() not implemented for HttpInstaller")
}

// Override link because we want to use HttpInstaller.Path() not base.Path()
func (i *HTTPInstaller) link(from string) error {
	debug("symlinking %s to %s", from, i.Path())
	return os.Symlink(from, i.Path())
}

// Path is overridden because we want to join on the plugin name not the file name
func (i HTTPInstaller) Path() string {
	if i.base.Source == "" {
		return ""
	}
	return filepath.Join(i.base.HelmHome.Plugins(), i.PluginName)
}
