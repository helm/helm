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

package installer

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/cache"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

// ArtifactInstaller installs plugins using the unified artifact downloader.
type ArtifactInstaller struct {
	CacheDir   string
	PluginName string
	base
	downloader *downloader.Downloader
	settings   *cli.EnvSettings
	version    string // Version constraint for VCS sources
	// Cached data to avoid duplicate downloads
	pluginData []byte
	provData   []byte
}

// NewArtifactInstaller creates a new ArtifactInstaller.
func NewArtifactInstaller(source string) (*ArtifactInstaller, error) {
	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	settings := cli.New()
	downloader := &downloader.Downloader{}

	i := &ArtifactInstaller{
		CacheDir:   helmpath.CachePath("plugins", key),
		PluginName: stripPluginName(filepath.Base(source)),
		base:       newBase(source),
		downloader: downloader,
		settings:   settings,
	}
	return i, nil
}

// Install downloads and installs the plugin.
func (i *ArtifactInstaller) Install() error {
	// Download plugin using unified downloader
	if i.pluginData == nil {
		tempDir := i.CacheDir + "-download"
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return fmt.Errorf("failed to create temp download directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		pluginPath, _, err := i.downloader.Download(i.Source, i.version, tempDir, downloader.TypePlugin)
		if err != nil {
			return fmt.Errorf("failed to download plugin: %w", err)
		}

		// Read the downloaded data
		data, err := os.ReadFile(pluginPath)
		if err != nil {
			return fmt.Errorf("failed to read downloaded plugin: %w", err)
		}
		i.pluginData = data

		// Try to get provenance data
		provPath := pluginPath + ".prov"
		if provData, err := os.ReadFile(provPath); err == nil {
			i.provData = provData
		}
	}

	// Extract metadata to get the actual plugin name and version
	metadata, err := plugin.ExtractTgzPluginMetadata(bytes.NewReader(i.pluginData))
	if err != nil {
		return fmt.Errorf("failed to extract plugin metadata from tarball: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version)

	// Save the original tarball to plugins directory
	tarballPath := helmpath.DataPath("plugins", filename)
	if err := os.MkdirAll(filepath.Dir(tarballPath), 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}
	if err := os.WriteFile(tarballPath, i.pluginData, 0644); err != nil {
		return fmt.Errorf("failed to save tarball: %w", err)
	}

	// Save prov file if we have the data
	if i.provData != nil {
		provPath := tarballPath + ".prov"
		if err := os.WriteFile(provPath, i.provData, 0644); err != nil {
			slog.Warn("failed to write provenance file", "path", provPath, "error", err)
		}
	}

	// Extract plugin to cache directory
	extractor, err := NewExtractor(filename)
	if err != nil {
		return err
	}

	if err := extractor.Extract(bytes.NewBuffer(i.pluginData), i.CacheDir); err != nil {
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

	return fs.CopyDir(src, i.Path())
}

// Update updates a plugin by reinstalling it.
func (i *ArtifactInstaller) Update() error {
	if err := os.RemoveAll(i.Path()); err != nil {
		return err
	}
	return i.Install()
}

// Path returns where the plugin will be installed.
func (i *ArtifactInstaller) Path() string {
	if i.Source == "" {
		return ""
	}
	return filepath.Join(i.settings.PluginsDirectory, i.PluginName)
}

// SupportsVerification returns true if the installer can verify plugins.
func (i *ArtifactInstaller) SupportsVerification() bool {
	return true // The unified downloader supports verification
}

// SetOptions sets additional options for the downloader.
func (i *ArtifactInstaller) SetOptions(options []getter.Option) {
	i.downloader.Options = append(i.downloader.Options, options...)
}

// SetVersion sets the version constraint for the plugin download.
func (i *ArtifactInstaller) SetVersion(version string) {
	i.version = version
}

// GetVerificationData returns cached plugin and provenance data for verification.
func (i *ArtifactInstaller) GetVerificationData() (archiveData, provData []byte, filename string, err error) {
	// Ensure data is cached
	if i.pluginData == nil {
		// Download plugin using unified downloader
		tempDir := i.CacheDir + "-verification"
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return nil, nil, "", fmt.Errorf("failed to create temp verification directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		pluginPath, _, err := i.downloader.Download(i.Source, i.version, tempDir, downloader.TypePlugin)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to download plugin for verification: %w", err)
		}

		// Read the downloaded data
		data, err := os.ReadFile(pluginPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to read downloaded plugin: %w", err)
		}
		i.pluginData = data

		// Try to get provenance data
		provPath := pluginPath + ".prov"
		if provData, err := os.ReadFile(provPath); err == nil {
			i.provData = provData
		}
	}

	// Extract metadata to get the filename
	metadata, err := plugin.ExtractTgzPluginMetadata(bytes.NewReader(i.pluginData))
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to extract plugin metadata from tarball: %w", err)
	}
	filename = fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version)

	return i.pluginData, i.provData, filename, nil
}
