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

package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/provenance"
)

// SignPlugin signs a plugin using the SHA256 hash of the tarball data.
//
// This is used when packaging and signing a plugin from tarball data.
// It creates a signature that includes the tarball hash and plugin metadata,
// allowing verification of the original tarball later.
func SignPlugin(tarballData []byte, filename string, signer *provenance.Signatory) (string, error) {
	// Extract plugin metadata from tarball data
	pluginMeta, err := ExtractTgzPluginMetadata(bytes.NewReader(tarballData))
	if err != nil {
		return "", fmt.Errorf("failed to extract plugin metadata: %w", err)
	}

	// Marshal plugin metadata to YAML bytes
	metadataBytes, err := yaml.Marshal(pluginMeta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal plugin metadata: %w", err)
	}

	// Use the generic provenance signing function
	return signer.ClearSign(tarballData, filename, metadataBytes)
}

// ExtractTgzPluginMetadata extracts plugin metadata from a gzipped tarball reader
func ExtractTgzPluginMetadata(r io.Reader) (*Metadata, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		// Look for plugin.yaml file
		if filepath.Base(header.Name) == "plugin.yaml" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}

			// Parse the plugin metadata
			metadata, err := loadMetadata(data)
			if err != nil {
				return nil, err
			}

			return metadata, nil
		}
	}

	return nil, errors.New("plugin.yaml not found in tarball")
}

// parsePluginMessageBlock parses a signed message block to extract plugin metadata and checksums
func parsePluginMessageBlock(data []byte) (*Metadata, *provenance.SumCollection, error) {
	sc := &provenance.SumCollection{}

	// We only need the checksums for verification, not the full metadata
	if err := provenance.ParseMessageBlock(data, nil, sc); err != nil {
		return nil, sc, err
	}
	return nil, sc, nil
}

// CreatePluginTarball creates a gzipped tarball from a plugin directory
func CreatePluginTarball(sourceDir, pluginName string, w io.Writer) error {
	gzw := gzip.NewWriter(w)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Use the plugin name as the base directory in the tarball
	baseDir := pluginName

	// Walk the directory tree
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Update the name to be relative to the source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Include the base directory name in the tarball
		header.Name = filepath.Join(baseDir, relPath)

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a regular file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})
}
