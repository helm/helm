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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/helmpath"
)

// ErrPluginNotADirectory indicates that the plugin path is not a directory.
var ErrPluginNotADirectory = errors.New("expected plugin to be a directory (containing a file 'plugin.yaml')")

// LocalInstaller installs plugins from the filesystem.
type LocalInstaller struct {
	base
	settings   *cli.EnvSettings
	isArchive  bool
	extractor  Extractor
	pluginData []byte // Cached plugin data
	provData   []byte // Cached provenance data
}

// NewLocalInstaller creates a new LocalInstaller.
func NewLocalInstaller(source string) (*LocalInstaller, error) {
	src, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("unable to get absolute path to plugin: %w", err)
	}

	settings := cli.New()

	i := &LocalInstaller{
		base:     newBase(src),
		settings: settings,
	}

	// Check if source is an archive
	if isLocalArchive(src) {
		i.isArchive = true
		extractor, err := NewExtractor(src)
		if err != nil {
			return nil, fmt.Errorf("unsupported archive format: %w", err)
		}
		i.extractor = extractor
	}

	return i, nil
}

// isLocalArchive checks if the file is a supported archive format
func isLocalArchive(path string) bool {
	for suffix := range Extractors {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// Install creates a symlink to the plugin directory.
//
// Implements Installer.
func (i *LocalInstaller) Install() error {
	if i.isArchive {
		return i.installFromArchive()
	}
	return i.installFromDirectory()
}

// installFromDirectory creates a symlink to the plugin directory
func (i *LocalInstaller) installFromDirectory() error {
	stat, err := os.Stat(i.Source)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return ErrPluginNotADirectory
	}

	if !isPlugin(i.Source) {
		return ErrMissingMetadata
	}
	slog.Debug("symlinking", "source", i.Source, "path", i.Path())
	return os.Symlink(i.Source, i.Path())
}

// installFromArchive extracts and installs a plugin from a tarball
func (i *LocalInstaller) installFromArchive() error {
	// Read the archive file
	data, err := os.ReadFile(i.Source)
	if err != nil {
		return fmt.Errorf("failed to read archive: %w", err)
	}

	// Copy the original tarball to plugins directory for verification
	// Extract metadata to get the actual plugin name and version
	metadata, err := plugin.ExtractTgzPluginMetadata(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to extract plugin metadata from tarball: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version)
	tarballPath := helmpath.DataPath("plugins", filename)
	if err := os.MkdirAll(filepath.Dir(tarballPath), 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}
	if err := os.WriteFile(tarballPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save tarball: %w", err)
	}

	// Check for and copy .prov file if it exists
	provSource := i.Source + ".prov"
	if provData, err := os.ReadFile(provSource); err == nil {
		provPath := tarballPath + ".prov"
		if err := os.WriteFile(provPath, provData, 0644); err != nil {
			slog.Debug("failed to save provenance file", "error", err)
		}
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "helm-plugin-extract-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the archive
	buffer := bytes.NewBuffer(data)
	if err := i.extractor.Extract(buffer, tempDir); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Plugin directory should be named after the plugin at the archive root
	pluginName := stripPluginName(filepath.Base(i.Source))
	pluginDir := filepath.Join(tempDir, pluginName)
	if _, err = os.Stat(filepath.Join(pluginDir, "plugin.yaml")); err != nil {
		return fmt.Errorf("plugin.yaml not found in expected directory %s: %w", pluginDir, err)
	}

	// Copy to the final destination
	slog.Debug("copying", "source", pluginDir, "path", i.Path())
	return fs.CopyDir(pluginDir, i.Path())
}

// Update updates a local repository
func (i *LocalInstaller) Update() error {
	slog.Debug("local repository is auto-updated")
	return nil
}

// Path is overridden to handle archive plugin names properly
func (i *LocalInstaller) Path() string {
	if i.Source == "" {
		return ""
	}

	pluginName := filepath.Base(i.Source)
	if i.isArchive {
		// Strip archive extension to get plugin name
		pluginName = stripPluginName(pluginName)
	}

	return filepath.Join(i.settings.PluginsDirectory, pluginName)
}

// SupportsVerification returns true if the local installer can verify plugins
func (i *LocalInstaller) SupportsVerification() bool {
	// Only support verification for local tarball files
	return i.isArchive
}

// GetVerificationData loads plugin and provenance data from local files for verification
func (i *LocalInstaller) GetVerificationData() (archiveData, provData []byte, filename string, err error) {
	if !i.SupportsVerification() {
		return nil, nil, "", fmt.Errorf("verification not supported for directories")
	}

	// Read and cache the plugin archive file
	if i.pluginData == nil {
		i.pluginData, err = os.ReadFile(i.Source)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to read plugin file: %w", err)
		}
	}

	// Read and cache the provenance file if it exists
	if i.provData == nil {
		provFile := i.Source + ".prov"
		i.provData, err = os.ReadFile(provFile)
		if err != nil {
			if os.IsNotExist(err) {
				// If provenance file doesn't exist, set provData to nil
				// The verification logic will handle this gracefully
				i.provData = nil
			} else {
				// If file exists but can't be read (permissions, etc), return error
				return nil, nil, "", fmt.Errorf("failed to access provenance file %s: %w", provFile, err)
			}
		}
	}

	return i.pluginData, i.provData, filepath.Base(i.Source), nil
}
