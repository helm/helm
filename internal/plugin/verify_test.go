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

	"helm.sh/helm/v4/pkg/provenance"
)

const testKeyFile = "../../pkg/cmd/testdata/helm-test-key.secret"
const testPubFile = "../../pkg/cmd/testdata/helm-test-key.pub"

const testPluginYAML = `apiVersion: v1
name: test-plugin
type: cli/v1
runtime: subprocess
version: 1.0.0
runtimeConfig:
  platformCommand:
    - command: echo`

func TestVerifyPlugin(t *testing.T) {
	// Create a test plugin and sign it
	tempDir := t.TempDir()

	// Create plugin directory
	pluginDir := filepath.Join(tempDir, "verify-test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tarball
	tarballPath := filepath.Join(tempDir, "verify-test-plugin.tar.gz")
	tarFile, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := CreatePluginTarball(pluginDir, "test-plugin", tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Sign the plugin with source directory
	signer, err := provenance.NewFromKeyring(testKeyFile, "helm-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.DecryptKey(func(_ string) ([]byte, error) {
		return []byte(""), nil
	}); err != nil {
		t.Fatal(err)
	}

	// Read the tarball data
	tarballData, err := os.ReadFile(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := SignPlugin(tarballData, filepath.Base(tarballPath), signer)
	if err != nil {
		t.Fatal(err)
	}

	// Write the signature to .prov file
	provFile := tarballPath + ".prov"
	if err := os.WriteFile(provFile, []byte(sig), 0644); err != nil {
		t.Fatal(err)
	}

	// Read the files for verification
	archiveData, err := os.ReadFile(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	provData, err := os.ReadFile(provFile)
	if err != nil {
		t.Fatal(err)
	}

	// Now verify the plugin
	verification, err := VerifyPlugin(archiveData, provData, filepath.Base(tarballPath), testPubFile)
	if err != nil {
		t.Fatalf("Failed to verify plugin: %v", err)
	}

	// Check verification results
	if verification.SignedBy == nil {
		t.Error("SignedBy is nil")
	}

	if verification.FileName != "verify-test-plugin.tar.gz" {
		t.Errorf("Expected filename 'verify-test-plugin.tar.gz', got %s", verification.FileName)
	}

	if verification.FileHash == "" {
		t.Error("FileHash is empty")
	}
}

func TestVerifyPluginBadSignature(t *testing.T) {
	tempDir := t.TempDir()

	// Create a plugin tarball
	pluginDir := filepath.Join(tempDir, "bad-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	tarballPath := filepath.Join(tempDir, "bad-plugin.tar.gz")
	tarFile, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := CreatePluginTarball(pluginDir, "test-plugin", tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Create a bad signature (just some text)
	badSig := `-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

This is not a real signature
-----BEGIN PGP SIGNATURE-----

InvalidSignatureData

-----END PGP SIGNATURE-----`

	provFile := tarballPath + ".prov"
	if err := os.WriteFile(provFile, []byte(badSig), 0644); err != nil {
		t.Fatal(err)
	}

	// Read the files
	archiveData, err := os.ReadFile(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	provData, err := os.ReadFile(provFile)
	if err != nil {
		t.Fatal(err)
	}

	// Try to verify - should fail
	_, err = VerifyPlugin(archiveData, provData, filepath.Base(tarballPath), testPubFile)
	if err == nil {
		t.Error("Expected verification to fail with bad signature")
	}
}

func TestVerifyPluginMissingProvenance(t *testing.T) {
	tempDir := t.TempDir()
	tarballPath := filepath.Join(tempDir, "no-prov.tar.gz")

	// Create a minimal tarball
	if err := os.WriteFile(tarballPath, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	// Read the tarball data
	archiveData, err := os.ReadFile(tarballPath)
	if err != nil {
		t.Fatal(err)
	}

	// Try to verify with empty provenance data
	_, err = VerifyPlugin(archiveData, nil, filepath.Base(tarballPath), testPubFile)
	if err == nil {
		t.Error("Expected verification to fail with empty provenance data")
	}
}

func TestVerifyPluginMalformedData(t *testing.T) {
	// Test with malformed tarball data - should fail
	malformedData := []byte("not a tarball")
	provData := []byte("fake provenance")

	_, err := VerifyPlugin(malformedData, provData, "malformed.tar.gz", testPubFile)
	if err == nil {
		t.Error("Expected malformed data verification to fail, but it succeeded")
	}
}
