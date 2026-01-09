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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/cache"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/registry"
)

// Ensure OCIInstaller implements Verifier
var _ Verifier = (*OCIInstaller)(nil)

// OCIInstaller installs plugins from OCI registries
type OCIInstaller struct {
	CacheDir   string
	PluginName string
	base
	settings *cli.EnvSettings
	getter   getter.Getter
	// Cached data to avoid duplicate downloads
	pluginData []byte
	provData   []byte
}

// NewOCIInstaller creates a new OCIInstaller with optional getter options
func NewOCIInstaller(source string, options ...getter.Option) (*OCIInstaller, error) {
	// Extract plugin name from OCI reference using robust registry parsing
	pluginName, err := registry.GetPluginName(source)
	if err != nil {
		return nil, err
	}

	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	settings := cli.New()

	// Always add plugin artifact type and any provided options
	pluginOptions := append([]getter.Option{getter.WithArtifactType("plugin")}, options...)
	getterProvider, err := getter.NewOCIGetter(pluginOptions...)
	if err != nil {
		return nil, err
	}

	i := &OCIInstaller{
		CacheDir:   helmpath.CachePath("plugins", key),
		PluginName: pluginName,
		base:       newBase(source),
		settings:   settings,
		getter:     getterProvider,
	}
	return i, nil
}

// Install downloads and installs a plugin from OCI registry
// Implements Installer.
func (i *OCIInstaller) Install() error {
	slog.Debug("pulling OCI plugin", "source", i.Source)

	// Ensure plugin data is cached
	if i.pluginData == nil {
		pluginData, err := i.getter.Get(i.Source)
		if err != nil {
			return fmt.Errorf("failed to pull plugin from %s: %w", i.Source, err)
		}
		i.pluginData = pluginData.Bytes()
	}

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
		provSource := i.Source + ".prov"
		if provData, err := i.getter.Get(provSource); err == nil {
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

	// Check if this is a gzip compressed file
	if len(i.pluginData) < 2 || i.pluginData[0] != 0x1f || i.pluginData[1] != 0x8b {
		return fmt.Errorf("plugin data is not a gzip compressed archive")
	}

	// Create cache directory
	if err := os.MkdirAll(i.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Extract as gzipped tar
	if err := extractTarGz(bytes.NewReader(i.pluginData), i.CacheDir); err != nil {
		return fmt.Errorf("failed to extract plugin: %w", err)
	}

	// Verify plugin.yaml exists - check root and subdirectories
	pluginDir := i.CacheDir
	if !isPlugin(pluginDir) {
		// Check if plugin.yaml is in a subdirectory
		entries, err := os.ReadDir(i.CacheDir)
		if err != nil {
			return err
		}

		foundPluginDir := ""
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(i.CacheDir, entry.Name())
				if isPlugin(subDir) {
					foundPluginDir = subDir
					break
				}
			}
		}

		if foundPluginDir == "" {
			return ErrMissingMetadata
		}

		// Use the subdirectory as the plugin directory
		pluginDir = foundPluginDir
	}

	// Copy from cache to final destination
	src, err := filepath.Abs(pluginDir)
	if err != nil {
		return err
	}

	slog.Debug("copying", "source", src, "path", i.Path())
	return fs.CopyDir(src, i.Path())
}

// Update updates a plugin by reinstalling it
func (i *OCIInstaller) Update() error {
	// For OCI, update means removing the old version and installing the new one
	if err := os.RemoveAll(i.Path()); err != nil {
		return err
	}
	return i.Install()
}

// Path is where the plugin will be installed
func (i OCIInstaller) Path() string {
	if i.Source == "" {
		return ""
	}
	return filepath.Join(i.settings.PluginsDirectory, i.PluginName)
}

// extractTarGz extracts a gzipped tar archive to a directory
func extractTarGz(r io.Reader, targetDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return extractTar(gzr, targetDir)
}

// extractTar extracts a tar archive to a directory
func extractTar(r io.Reader, targetDir string) error {
	tarReader := tar.NewReader(r)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		path, err := cleanJoin(targetDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			// Skip these
			continue
		default:
			return fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

// SupportsVerification returns true since OCI plugins can be verified
func (i *OCIInstaller) SupportsVerification() bool {
	return true
}

// GetVerificationData downloads and caches plugin and provenance data from OCI registry for verification
func (i *OCIInstaller) GetVerificationData() (archiveData, provData []byte, filename string, err error) {
	slog.Debug("getting verification data for OCI plugin", "source", i.Source)

	// Download plugin data once and cache it
	if i.pluginData == nil {
		pluginDataBuffer, err := i.getter.Get(i.Source)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to pull plugin from %s: %w", i.Source, err)
		}
		i.pluginData = pluginDataBuffer.Bytes()
	}

	// Download prov data once and cache it if available
	if i.provData == nil {
		provSource := i.Source + ".prov"
		// Calling getter.Get again is reasonable because: 1. The OCI registry client already optimizes the underlying network calls
		// 2. Both calls use the same underlying manifest and memory store 3. The second .prov call is very fast since the data is already pulled
		provDataBuffer, err := i.getter.Get(provSource)
		if err != nil {
			// If provenance file doesn't exist, set provData to nil
			// The verification logic will handle this gracefully
			i.provData = nil
		} else {
			i.provData = provDataBuffer.Bytes()
		}
	}

	// Extract metadata to get the filename
	metadata, err := plugin.ExtractTgzPluginMetadata(bytes.NewReader(i.pluginData))
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to extract plugin metadata from tarball: %w", err)
	}
	filename = fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version)

	slog.Debug("got verification data for OCI plugin", "filename", filename)
	return i.pluginData, i.provData, filename, nil
}
