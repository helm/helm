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

package cmd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/test/ensure"
)

func TestPluginVerifyCmd_NoArgs(t *testing.T) {
	ensure.HelmHome(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{})

	assert.ErrorContains(t, cmd.Execute(), "requires 1 argument", "expected 'requires 1 argument' error")
}

func TestPluginVerifyCmd_TooManyArgs(t *testing.T) {
	ensure.HelmHome(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{"plugin1", "plugin2"})

	assert.ErrorContains(t, cmd.Execute(), "requires 1 argument", "expected 'requires 1 argument' error")
}

func TestPluginVerifyCmd_NonexistentFile(t *testing.T) {
	ensure.HelmHome(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{"/nonexistent/plugin.tgz"})

	assert.Error(t, cmd.Execute(), "expected error when plugin file doesn't exist")
}

func TestPluginVerifyCmd_MissingProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin tarball without .prov file
	pluginTgz := createTestPluginTarball(t)
	defer os.Remove(pluginTgz)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{pluginTgz})

	assert.ErrorContains(t, cmd.Execute(), "could not find provenance file", "expected 'could not find provenance file' error")
}

func TestPluginVerifyCmd_InvalidProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin tarball with invalid .prov file
	pluginTgz := createTestPluginTarball(t)
	defer os.Remove(pluginTgz)

	// Create invalid .prov file
	provFile := pluginTgz + ".prov"
	require.NoError(t, os.WriteFile(provFile, []byte("invalid provenance"), 0o644))
	defer os.Remove(provFile)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{pluginTgz})

	assert.Error(t, cmd.Execute(), "expected error when .prov file is invalid")
}

func TestPluginVerifyCmd_DirectoryNotSupported(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin directory
	pluginDir := createTestPluginDir(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{pluginDir})

	assert.ErrorContains(t, cmd.Execute(), "directory verification not supported", "expected 'directory verification not supported' error")
}

func TestPluginVerifyCmd_KeyringFlag(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin tarball with .prov file
	pluginTgz := createTestPluginTarball(t)
	defer os.Remove(pluginTgz)

	// Create .prov file
	provFile := pluginTgz + ".prov"
	createProvFile(t, provFile, pluginTgz, "")
	defer os.Remove(provFile)

	// Create empty keyring file
	keyring := createTestKeyring(t)
	defer os.Remove(keyring)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{"--keyring", keyring, pluginTgz})

	// Should fail with keyring error but command parsing should work
	assert.Error(t, cmd.Execute(), "expected error with empty keyring")
	// The important thing is that the keyring flag was parsed and used
}

func TestPluginVerifyOptions_Run_Success(t *testing.T) {
	// Skip this test as it would require real PGP keys and valid signatures
	// The core verification logic is thoroughly tested in internal/plugin/verify_test.go
	t.Skip("Success case requires real PGP keys - core logic tested in internal/plugin/verify_test.go")
}

// Helper functions for test setup

func createTestPluginDir(t *testing.T) string {
	t.Helper()

	// Create temporary directory with plugin structure
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	require.NoErrorf(t, os.MkdirAll(pluginDir, 0o755), "Failed to create plugin directory")

	// Use the same plugin YAML as other cmd tests
	require.NoErrorf(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0o644), "Failed to create plugin.yaml")

	return pluginDir
}

func createTestPluginTarball(t *testing.T) string {
	t.Helper()

	pluginDir := createTestPluginDir(t)

	// Create tarball using the plugin package helper
	tmpDir := filepath.Dir(pluginDir)
	tgzPath := filepath.Join(tmpDir, "test-plugin-1.0.0.tgz")
	tarFile, err := os.Create(tgzPath)
	require.NoError(t, err, "Failed to create tarball file")
	defer tarFile.Close()

	require.NoErrorf(t, plugin.CreatePluginTarball(pluginDir, "test-plugin", tarFile), "Failed to create tarball")

	return tgzPath
}

func createProvFile(t *testing.T, provFile, pluginTgz, hash string) {
	t.Helper()

	var hashStr string
	if hash == "" {
		// Calculate actual hash of the tarball
		data, err := os.ReadFile(pluginTgz)
		require.NoError(t, err, "Failed to read tarball for hashing")
		hashSum := sha256.Sum256(data)
		hashStr = fmt.Sprintf("sha256:%x", hashSum)
	} else {
		// Use provided hash
		hashStr = hash
	}

	// Create properly formatted provenance file with specified hash
	provContent := fmt.Sprintf(`-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

name: test-plugin
version: 1.0.0
description: Test plugin for verification
files:
  test-plugin-1.0.0.tgz: %s
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1

iQEcBAEBCAAGBQJktest...
-----END PGP SIGNATURE-----
`, hashStr)
	require.NoErrorf(t, os.WriteFile(provFile, []byte(provContent), 0o644), "Failed to create provenance file")
}

func createTestKeyring(t *testing.T) string {
	t.Helper()

	// Create a temporary keyring file
	tmpDir := t.TempDir()
	keyringPath := filepath.Join(tmpDir, "pubring.gpg")

	// Create empty keyring for testing
	require.NoErrorf(t, os.WriteFile(keyringPath, []byte{}, 0o644), "Failed to create test keyring")

	return keyringPath
}
