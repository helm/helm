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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/provenance"
)

func TestSignPlugin(t *testing.T) {
	// Create a test plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	// Create a plugin.yaml file
	pluginYAML := `apiVersion: v1
name: test-plugin
type: cli/v1
runtime: subprocess
version: 1.0.0
runtimeConfig:
  platformCommand:
    - command: echo`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0o644))

	// Create a tarball
	tarballPath := filepath.Join(tempDir, "test-plugin.tgz")
	tarFile, err := os.Create(tarballPath)
	require.NoError(t, err)
	if err := CreatePluginTarball(pluginDir, "test-plugin", tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Create a test key for signing
	keyring := "../../pkg/cmd/testdata/helm-test-key.secret"
	signer, err := provenance.NewFromKeyring(keyring, "helm-test")
	require.NoError(t, err)
	require.NoError(t, signer.DecryptKey(func(_ string) ([]byte, error) {
		return []byte(""), nil
	}))

	// Read the tarball data
	tarballData, err := os.ReadFile(tarballPath)
	require.NoError(t, err, "failed to read tarball")

	// Sign the plugin tarball
	sig, err := SignPlugin(tarballData, filepath.Base(tarballPath), signer)
	require.NoError(t, err, "failed to sign plugin")

	// Verify the signature contains the expected content
	assert.Contains(t, sig, "-----BEGIN PGP SIGNED MESSAGE-----", "signature does not contain PGP header")

	// Verify the tarball hash is in the signature
	expectedHash, err := provenance.DigestFile(tarballPath)
	require.NoError(t, err)
	// The signature should contain the tarball hash
	assert.Contains(t, sig, "sha256:"+expectedHash, "signature does not contain expected tarball hash: sha256:%s", expectedHash)
}
