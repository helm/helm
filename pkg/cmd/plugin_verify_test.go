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
	"strings"
	"testing"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/test/ensure"
)

func TestPluginVerifyCmd_NoArgs(t *testing.T) {
	ensure.HelmHome(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}
	if !strings.Contains(err.Error(), "requires 1 argument") {
		t.Errorf("expected 'requires 1 argument' error, got: %v", err)
	}
}

func TestPluginVerifyCmd_TooManyArgs(t *testing.T) {
	ensure.HelmHome(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{"plugin1", "plugin2"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when too many arguments provided")
	}
	if !strings.Contains(err.Error(), "requires 1 argument") {
		t.Errorf("expected 'requires 1 argument' error, got: %v", err)
	}
}

func TestPluginVerifyCmd_NonexistentFile(t *testing.T) {
	ensure.HelmHome(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{"/nonexistent/plugin.tgz"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when plugin file doesn't exist")
	}
}

func TestPluginVerifyCmd_MissingProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin tarball without .prov file
	pluginTgz := createTestPluginTarball(t)
	defer os.Remove(pluginTgz)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{pluginTgz})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when .prov file is missing")
	}
	if !strings.Contains(err.Error(), "could not find provenance file") {
		t.Errorf("expected 'could not find provenance file' error, got: %v", err)
	}
}

func TestPluginVerifyCmd_InvalidProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin tarball with invalid .prov file
	pluginTgz := createTestPluginTarball(t)
	defer os.Remove(pluginTgz)

	// Create invalid .prov file
	provFile := pluginTgz + ".prov"
	if err := os.WriteFile(provFile, []byte("invalid provenance"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(provFile)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{pluginTgz})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when .prov file is invalid")
	}
}

func TestPluginVerifyCmd_DirectoryNotSupported(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin directory
	pluginDir := createTestPluginDir(t)

	out := &bytes.Buffer{}
	cmd := newPluginVerifyCmd(out)
	cmd.SetArgs([]string{pluginDir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when verifying directory")
	}
	if !strings.Contains(err.Error(), "directory verification not supported") {
		t.Errorf("expected 'directory verification not supported' error, got: %v", err)
	}
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
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with empty keyring")
	}
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
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("Failed to create plugin directory: %v", err)
	}

	// Use the same plugin YAML as other cmd tests
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0644); err != nil {
		t.Fatalf("Failed to create plugin.yaml: %v", err)
	}

	return pluginDir
}

func createTestPluginTarball(t *testing.T) string {
	t.Helper()

	pluginDir := createTestPluginDir(t)

	// Create tarball using the plugin package helper
	tmpDir := filepath.Dir(pluginDir)
	tgzPath := filepath.Join(tmpDir, "test-plugin-1.0.0.tgz")
	tarFile, err := os.Create(tgzPath)
	if err != nil {
		t.Fatalf("Failed to create tarball file: %v", err)
	}
	defer tarFile.Close()

	if err := plugin.CreatePluginTarball(pluginDir, "test-plugin", tarFile); err != nil {
		t.Fatalf("Failed to create tarball: %v", err)
	}

	return tgzPath
}

func createProvFile(t *testing.T, provFile, pluginTgz, hash string) {
	t.Helper()

	var hashStr string
	if hash == "" {
		// Calculate actual hash of the tarball
		data, err := os.ReadFile(pluginTgz)
		if err != nil {
			t.Fatalf("Failed to read tarball for hashing: %v", err)
		}
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
	if err := os.WriteFile(provFile, []byte(provContent), 0644); err != nil {
		t.Fatalf("Failed to create provenance file: %v", err)
	}
}

func createTestKeyring(t *testing.T) string {
	t.Helper()

	// Create a temporary keyring file
	tmpDir := t.TempDir()
	keyringPath := filepath.Join(tmpDir, "pubring.gpg")

	// Create empty keyring for testing
	if err := os.WriteFile(keyringPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test keyring: %v", err)
	}

	return keyringPath
}
