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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/pkg/provenance"
)

// VerifyPlugin verifies a plugin tarball against a signature.
//
// This function verifies that a plugin tarball has a valid provenance file
// and that the provenance file is signed by a trusted entity.
func VerifyPlugin(pluginPath, keyring string) (*provenance.Verification, error) {
	// Verify the plugin path exists
	fi, err := os.Stat(pluginPath)
	if err != nil {
		return nil, err
	}

	// Only support tarball verification
	if fi.IsDir() {
		return nil, errors.New("directory verification not supported - only plugin tarballs can be verified")
	}

	// Verify it's a tarball
	if !isTarball(pluginPath) {
		return nil, errors.New("plugin file must be a gzipped tarball (.tar.gz or .tgz)")
	}

	// Look for provenance file
	provFile := pluginPath + ".prov"
	if _, err := os.Stat(provFile); err != nil {
		return nil, fmt.Errorf("could not find provenance file %s: %w", provFile, err)
	}

	// Create signatory from keyring
	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, err
	}

	return verifyPluginTarball(pluginPath, provFile, sig)
}

// verifyPluginTarball verifies a plugin tarball against its signature
func verifyPluginTarball(pluginPath, provPath string, sig *provenance.Signatory) (*provenance.Verification, error) {
	// Reuse chart verification logic from pkg/provenance
	return sig.Verify(pluginPath, provPath)
}

// isTarball checks if a file has a tarball extension
func isTarball(filename string) bool {
	return filepath.Ext(filename) == ".gz" || filepath.Ext(filename) == ".tgz"
}
