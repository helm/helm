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

	"helm.sh/helm/v4/internal/third_party/dep/fs"
)

// ErrPluginNotAFolder indicates that the plugin path is not a folder.
var ErrPluginNotAFolder = errors.New("expected plugin to be a folder")

// LocalInstaller installs plugins from the filesystem.
type LocalInstaller struct {
	base
	isArchive bool
	extractor Extractor
}

// NewLocalInstaller creates a new LocalInstaller.
func NewLocalInstaller(source string) (*LocalInstaller, error) {
	src, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("unable to get absolute path to plugin: %w", err)
	}
	i := &LocalInstaller{
		base: newBase(src),
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
		return ErrPluginNotAFolder
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

	// Detect where the plugin.yaml actually is
	pluginRoot, err := detectPluginRoot(tempDir)
	if err != nil {
		return err
	}

	// Copy to the final destination
	slog.Debug("copying", "source", pluginRoot, "path", i.Path())
	return fs.CopyDir(pluginRoot, i.Path())
}

// Path returns the path where the plugin will be installed.
// For archive sources, strips the version from the filename.
func (i *LocalInstaller) Path() string {
	if i.Source == "" {
		return ""
	}
	if i.isArchive {
		return filepath.Join(i.PluginsDirectory, stripPluginName(filepath.Base(i.Source)))
	}
	return filepath.Join(i.PluginsDirectory, filepath.Base(i.Source))
}

// Update updates a local repository
func (i *LocalInstaller) Update() error {
	slog.Debug("local repository is auto-updated")
	return nil
}
