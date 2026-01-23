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

package installer // import "helm.sh/helm/v4/internal/plugin/installer"

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/cache"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

// HTTPInstaller installs plugins from an archive served by a web server.
type HTTPInstaller struct {
	CacheDir   string
	PluginName string
	base
	settings  *cli.EnvSettings
	extractor Extractor
	getter    getter.Getter
	// Cached data to avoid duplicate downloads
	pluginData []byte
	provData   []byte
}

// NewHTTPInstaller creates a new HttpInstaller.
func NewHTTPInstaller(source string) (*HTTPInstaller, error) {
	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	settings := cli.New()

	extractor, err := NewExtractor(source)
	if err != nil {
		return nil, err
	}

	get, err := getter.All(new(cli.EnvSettings)).ByScheme("http")
	if err != nil {
		return nil, err
	}

	i := &HTTPInstaller{
		CacheDir:   helmpath.CachePath("plugins", key),
		PluginName: stripPluginName(filepath.Base(source)),
		base:       newBase(source),
		settings:   settings,
		extractor:  extractor,
		getter:     get,
	}
	return i, nil
}

// Install downloads and extracts the tarball into the cache directory
// and installs into the plugin directory.
//
// Implements Installer.
func (i *HTTPInstaller) Install() error {
	// Ensure plugin data is cached
	if i.pluginData == nil {
		pluginData, err := i.getter.Get(i.Source)
		if err != nil {
			return err
		}
		i.pluginData = pluginData.Bytes()
	}

	// Save the original tarball to plugins directory for verification
	// Extract metadata to get the actual plugin name and version
	metadata, err := plugin.ExtractTgzPluginMetadata(bytes.NewReader(i.pluginData))
	if err != nil {
		return fmt.Errorf("failed to extract plugin metadata from tarball: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version)
	tarballPath := helmpath.DataPath("plugins", filename)
	if err := os.MkdirAll(filepath.Dir(tarballPath), 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}
	if err := os.WriteFile(tarballPath, i.pluginData, 0644); err != nil {
		return fmt.Errorf("failed to save tarball: %w", err)
	}

	// Ensure prov data is cached if available
	if i.provData == nil {
		// Try to download .prov file if it exists
		provURL := i.Source + ".prov"
		if provData, err := i.getter.Get(provURL); err == nil {
			i.provData = provData.Bytes()
		}
	}

	// Save prov file if we have the data
	if i.provData != nil {
		provPath := tarballPath + ".prov"
		if err := os.WriteFile(provPath, i.provData, 0644); err != nil {
			slog.Debug("failed to save provenance file", "error", err)
		}
	}

	if err := i.extractor.Extract(bytes.NewBuffer(i.pluginData), i.CacheDir); err != nil {
		return fmt.Errorf("extracting files from archive: %w", err)
	}

	// Detect where the plugin.yaml actually is
	pluginRoot, err := detectPluginRoot(i.CacheDir)
	if err != nil {
		return err
	}

	// Validate plugin structure if needed
	if err := validatePluginName(pluginRoot, i.PluginName); err != nil {
		return err
	}

	src, err := filepath.Abs(pluginRoot)
	if err != nil {
		return err
	}

	slog.Debug("copying", "source", src, "path", i.Path())
	return fs.CopyDir(src, i.Path())
}

// Update updates a local repository
// Not implemented for now since tarball most likely will be packaged by version
func (i *HTTPInstaller) Update() error {
	return fmt.Errorf("method Update() not implemented for HttpInstaller")
}

// Path is overridden because we want to join on the plugin name not the file name
func (i HTTPInstaller) Path() string {
	if i.Source == "" {
		return ""
	}
	return filepath.Join(i.settings.PluginsDirectory, i.PluginName)
}

// SupportsVerification returns true if the HTTP installer can verify plugins
func (i *HTTPInstaller) SupportsVerification() bool {
	// Only support verification for tarball URLs
	return strings.HasSuffix(i.Source, ".tgz") || strings.HasSuffix(i.Source, ".tar.gz")
}

// GetVerificationData returns cached plugin and provenance data for verification
func (i *HTTPInstaller) GetVerificationData() (archiveData, provData []byte, filename string, err error) {
	if !i.SupportsVerification() {
		return nil, nil, "", fmt.Errorf("verification not supported for this source")
	}

	// Download plugin data once and cache it
	if i.pluginData == nil {
		data, err := i.getter.Get(i.Source)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to download plugin: %w", err)
		}
		i.pluginData = data.Bytes()
	}

	// Download prov data once and cache it if available
	if i.provData == nil {
		provData, err := i.getter.Get(i.Source + ".prov")
		if err != nil {
			// If provenance file doesn't exist, set provData to nil
			// The verification logic will handle this gracefully
			i.provData = nil
		} else {
			i.provData = provData.Bytes()
		}
	}

	return i.pluginData, i.provData, filepath.Base(i.Source), nil
}
