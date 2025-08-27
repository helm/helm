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
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/provenance"
)

func TestSignPlugin(t *testing.T) {
	// Create a test plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plugin.yaml file
	pluginYAML := `apiVersion: v1
name: test-plugin
type: cli/v1
runtime: subprocess
version: 1.0.0
runtimeConfig:
  platformCommand:
    - command: echo`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a tarball
	tarballPath := filepath.Join(tempDir, "test-plugin.tgz")
	tarFile, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreatePluginTarball(pluginDir, "test-plugin", tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()

	// Create a test key for signing
	keyring := "../../pkg/cmd/testdata/helm-test-key.secret"
	signer, err := provenance.NewFromKeyring(keyring, "helm-test")
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
		t.Fatalf("failed to read tarball: %v", err)
	}

	// Sign the plugin tarball
	sig, err := SignPlugin(tarballData, filepath.Base(tarballPath), signer)
	if err != nil {
		t.Fatalf("failed to sign plugin: %v", err)
	}

	// Verify the signature contains the expected content
	if !strings.Contains(sig, "-----BEGIN PGP SIGNED MESSAGE-----") {
		t.Error("signature does not contain PGP header")
	}

	// Verify the tarball hash is in the signature
	expectedHash, err := provenance.DigestFile(tarballPath)
	if err != nil {
		t.Fatal(err)
	}
	// The signature should contain the tarball hash
	if !strings.Contains(sig, "sha256:"+expectedHash) {
		t.Errorf("signature does not contain expected tarball hash: sha256:%s", expectedHash)
	}
}
