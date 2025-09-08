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
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/test/ensure"
)

func TestInstallWithOptions_VerifyMissingProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a temporary plugin tarball without .prov file
	pluginDir := createTestPluginDir(t)
	pluginTgz := createTarballFromPluginDir(t, pluginDir)
	defer os.Remove(pluginTgz)

	// Create local installer
	installer, err := NewLocalInstaller(pluginTgz)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Capture stderr to check warning message
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Install with verification enabled (should warn but succeed)
	result, err := InstallWithOptions(installer, Options{Verify: true, Keyring: "dummy"})

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should succeed with nil result (no verification performed)
	if err != nil {
		t.Fatalf("Expected installation to succeed despite missing .prov file, got error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil verification result when .prov file is missing, got: %+v", result)
	}

	// Should contain warning message
	expectedWarning := "WARNING: No provenance file found for plugin"
	if !strings.Contains(output, expectedWarning) {
		t.Errorf("Expected warning message '%s' in output, got: %s", expectedWarning, output)
	}

	// Plugin should be installed
	if _, err := os.Stat(installer.Path()); os.IsNotExist(err) {
		t.Errorf("Plugin should be installed at %s", installer.Path())
	}
}

func TestInstallWithOptions_VerifyWithValidProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a temporary plugin tarball with valid .prov file
	pluginDir := createTestPluginDir(t)
	pluginTgz := createTarballFromPluginDir(t, pluginDir)

	provFile := pluginTgz + ".prov"
	createProvFile(t, provFile, pluginTgz, "")
	defer os.Remove(provFile)

	// Create keyring with test key (empty for testing)
	keyring := createTestKeyring(t)
	defer os.Remove(keyring)

	// Create local installer
	installer, err := NewLocalInstaller(pluginTgz)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Install with verification enabled
	// This will fail signature verification but pass hash validation
	result, err := InstallWithOptions(installer, Options{Verify: true, Keyring: keyring})

	// Should fail due to invalid signature (empty keyring) but we test that it gets past the hash check
	if err == nil {
		t.Fatalf("Expected installation to fail with empty keyring")
	}
	if !strings.Contains(err.Error(), "plugin verification failed") {
		t.Errorf("Expected plugin verification failed error, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil verification result when verification fails, got: %+v", result)
	}

	// Plugin should not be installed due to verification failure
	if _, err := os.Stat(installer.Path()); !os.IsNotExist(err) {
		t.Errorf("Plugin should not be installed when verification fails")
	}
}

func TestInstallWithOptions_VerifyWithInvalidProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create a temporary plugin tarball with invalid .prov file
	pluginDir := createTestPluginDir(t)
	pluginTgz := createTarballFromPluginDir(t, pluginDir)
	defer os.Remove(pluginTgz)

	provFile := pluginTgz + ".prov"
	createProvFileInvalidFormat(t, provFile)
	defer os.Remove(provFile)

	// Create keyring with test key
	keyring := createTestKeyring(t)
	defer os.Remove(keyring)

	// Create local installer
	installer, err := NewLocalInstaller(pluginTgz)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Install with verification enabled (should fail)
	result, err := InstallWithOptions(installer, Options{Verify: true, Keyring: keyring})

	// Should fail with verification error
	if err == nil {
		t.Fatalf("Expected installation with invalid .prov file to fail")
	}
	if result != nil {
		t.Errorf("Expected nil verification result when verification fails, got: %+v", result)
	}

	// Should contain verification failure message
	expectedError := "plugin verification failed"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error message '%s', got: %s", expectedError, err.Error())
	}

	// Plugin should not be installed
	if _, err := os.Stat(installer.Path()); !os.IsNotExist(err) {
		t.Errorf("Plugin should not be installed when verification fails")
	}
}

func TestInstallWithOptions_NoVerifyRequested(t *testing.T) {
	ensure.HelmHome(t)

	// Create a temporary plugin tarball without .prov file
	pluginDir := createTestPluginDir(t)
	pluginTgz := createTarballFromPluginDir(t, pluginDir)
	defer os.Remove(pluginTgz)

	// Create local installer
	installer, err := NewLocalInstaller(pluginTgz)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Install without verification (should succeed without any verification)
	result, err := InstallWithOptions(installer, Options{Verify: false})

	// Should succeed with no verification
	if err != nil {
		t.Fatalf("Expected installation without verification to succeed, got error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil verification result when verification is disabled, got: %+v", result)
	}

	// Plugin should be installed
	if _, err := os.Stat(installer.Path()); os.IsNotExist(err) {
		t.Errorf("Plugin should be installed at %s", installer.Path())
	}
}

func TestInstallWithOptions_VerifyDirectoryNotSupported(t *testing.T) {
	ensure.HelmHome(t)

	// Create a directory-based plugin (not an archive)
	pluginDir := createTestPluginDir(t)

	// Create local installer for directory
	installer, err := NewLocalInstaller(pluginDir)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Install with verification should fail (directories don't support verification)
	result, err := InstallWithOptions(installer, Options{Verify: true, Keyring: "dummy"})

	// Should fail with verification not supported error
	if err == nil {
		t.Fatalf("Expected installation to fail with verification not supported error")
	}
	if !strings.Contains(err.Error(), "--verify is only supported for plugin tarballs") {
		t.Errorf("Expected verification not supported error, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil verification result when verification fails, got: %+v", result)
	}
}

func TestInstallWithOptions_VerifyMismatchedProvenance(t *testing.T) {
	ensure.HelmHome(t)

	// Create plugin tarball
	pluginDir := createTestPluginDir(t)
	pluginTgz := createTarballFromPluginDir(t, pluginDir)
	defer os.Remove(pluginTgz)

	provFile := pluginTgz + ".prov"
	// Create provenance file with wrong hash (for a different file)
	createProvFile(t, provFile, pluginTgz, "sha256:wronghash")
	defer os.Remove(provFile)

	// Create keyring with test key
	keyring := createTestKeyring(t)
	defer os.Remove(keyring)

	// Create local installer
	installer, err := NewLocalInstaller(pluginTgz)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Install with verification should fail due to hash mismatch
	result, err := InstallWithOptions(installer, Options{Verify: true, Keyring: keyring})

	// Should fail with verification error
	if err == nil {
		t.Fatalf("Expected installation to fail with hash mismatch")
	}
	if !strings.Contains(err.Error(), "plugin verification failed") {
		t.Errorf("Expected plugin verification failed error, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil verification result when verification fails, got: %+v", result)
	}
}

func TestInstallWithOptions_VerifyProvenanceAccessError(t *testing.T) {
	ensure.HelmHome(t)

	// Create plugin tarball
	pluginDir := createTestPluginDir(t)
	pluginTgz := createTarballFromPluginDir(t, pluginDir)
	defer os.Remove(pluginTgz)

	// Create a .prov file but make it inaccessible (simulate permission error)
	provFile := pluginTgz + ".prov"
	if err := os.WriteFile(provFile, []byte("test"), 0000); err != nil {
		t.Fatalf("Failed to create inaccessible provenance file: %v", err)
	}
	defer os.Remove(provFile)

	// Create keyring
	keyring := createTestKeyring(t)
	defer os.Remove(keyring)

	// Create local installer
	installer, err := NewLocalInstaller(pluginTgz)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	defer os.RemoveAll(installer.Path())

	// Install with verification should fail due to access error
	result, err := InstallWithOptions(installer, Options{Verify: true, Keyring: keyring})

	// Should fail with access error (either at stat level or during verification)
	if err == nil {
		t.Fatalf("Expected installation to fail with provenance file access error")
	}
	// The error could be either "failed to access provenance file" or "plugin verification failed"
	// depending on when the permission error occurs
	if !strings.Contains(err.Error(), "failed to access provenance file") &&
		!strings.Contains(err.Error(), "plugin verification failed") {
		t.Errorf("Expected provenance file access or verification error, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil verification result when verification fails, got: %+v", result)
	}
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

	// Create plugin.yaml using the standardized v1 format
	pluginYaml := `apiVersion: v1
name: test-plugin
type: cli/v1
runtime: subprocess
version: 1.0.0
runtimeConfig:
  platformCommand:
    - command: echo`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYaml), 0644); err != nil {
		t.Fatalf("Failed to create plugin.yaml: %v", err)
	}

	return pluginDir
}

func createTarballFromPluginDir(t *testing.T, pluginDir string) string {
	t.Helper()

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
		// Calculate actual hash of the tarball for realistic testing
		data, err := os.ReadFile(pluginTgz)
		if err != nil {
			t.Fatalf("Failed to read tarball for hashing: %v", err)
		}
		hashSum := sha256.Sum256(data)
		hashStr = fmt.Sprintf("sha256:%x", hashSum)
	} else {
		// Use provided hash (could be wrong for testing)
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

func createProvFileInvalidFormat(t *testing.T, provFile string) {
	t.Helper()

	// Create an invalid provenance file (not PGP signed format)
	invalidProv := "This is not a valid PGP signed message"
	if err := os.WriteFile(provFile, []byte(invalidProv), 0644); err != nil {
		t.Fatalf("Failed to create invalid provenance file: %v", err)
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
