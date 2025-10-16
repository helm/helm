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
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign" //nolint

	"helm.sh/helm/v4/pkg/helmpath"
)

// SigningInfo contains information about a plugin's signing status
type SigningInfo struct {
	// Status can be:
	// - "local dev": Plugin is a symlink (development mode)
	// - "unsigned": No provenance file found
	// - "invalid provenance": Provenance file is malformed
	// - "mismatched provenance": Provenance file does not match the installed tarball
	// - "signed": Valid signature exists for the installed tarball
	Status   string
	IsSigned bool // True if plugin has a valid signature (even if not verified against keyring)
}

// GetPluginSigningInfo returns signing information for an installed plugin
func GetPluginSigningInfo(metadata Metadata) (*SigningInfo, error) {
	pluginName := metadata.Name
	pluginDir := helmpath.DataPath("plugins", pluginName)

	// Check if plugin directory exists
	fi, err := os.Lstat(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("plugin %s not found: %w", pluginName, err)
	}

	// Check if it's a symlink (local development)
	if fi.Mode()&os.ModeSymlink != 0 {
		return &SigningInfo{
			Status:   "local dev",
			IsSigned: false,
		}, nil
	}

	// Find the exact tarball file for this plugin
	pluginsDir := helmpath.DataPath("plugins")
	tarballPath := filepath.Join(pluginsDir, fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version))
	if _, err := os.Stat(tarballPath); err != nil {
		return &SigningInfo{
			Status:   "unsigned",
			IsSigned: false,
		}, nil
	}

	// Check for .prov file associated with the tarball
	provFile := tarballPath + ".prov"
	provData, err := os.ReadFile(provFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &SigningInfo{
				Status:   "unsigned",
				IsSigned: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	// Parse the provenance file to check validity
	block, _ := clearsign.Decode(provData)
	if block == nil {
		return &SigningInfo{
			Status:   "invalid provenance",
			IsSigned: false,
		}, nil
	}

	// Check if provenance matches the actual tarball
	blockContent := string(block.Plaintext)
	if !validateProvenanceHash(blockContent, tarballPath) {
		return &SigningInfo{
			Status:   "mismatched provenance",
			IsSigned: false,
		}, nil
	}

	// We have a provenance file that is valid for this plugin
	// Without a keyring, we can't verify the signature, but we know:
	// 1. A .prov file exists
	// 2. It's a valid clearsigned document (cryptographically signed)
	// 3. The provenance contains valid checksums
	return &SigningInfo{
		Status:   "signed",
		IsSigned: true,
	}, nil
}

func validateProvenanceHash(blockContent string, tarballPath string) bool {
	// Parse provenance to get the expected hash
	_, sums, err := parsePluginMessageBlock([]byte(blockContent))
	if err != nil {
		return false
	}

	// Must have file checksums
	if len(sums.Files) == 0 {
		return false
	}

	// Calculate actual hash of the tarball
	actualHash, err := calculateFileHash(tarballPath)
	if err != nil {
		return false
	}

	// Check if the actual hash matches the expected hash in the provenance
	for filename, expectedHash := range sums.Files {
		if strings.Contains(filename, filepath.Base(tarballPath)) && expectedHash == actualHash {
			return true
		}
	}

	return false
}

// calculateFileHash calculates the SHA256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("sha256:%x", hasher.Sum(nil)), nil
}

// GetSigningInfoForPlugins returns signing info for multiple plugins
func GetSigningInfoForPlugins(plugins []Plugin) map[string]*SigningInfo {
	result := make(map[string]*SigningInfo)

	for _, p := range plugins {
		m := p.Metadata()

		info, err := GetPluginSigningInfo(m)
		if err != nil {
			// If there's an error, treat as unsigned
			result[m.Name] = &SigningInfo{
				Status:   "unknown",
				IsSigned: false,
			}
		} else {
			result[m.Name] = info
		}
	}

	return result
}
