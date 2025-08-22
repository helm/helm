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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Common plugin.yaml content for v1 format tests
const testPluginYAML = `apiVersion: v1
name: test-plugin
version: 1.0.0
type: cli/v1
runtime: subprocess
config:
  usage: test-plugin [flags]
  shortHelp: A test plugin
  longHelp: A test plugin for testing purposes
runtimeConfig:
  platformCommand:
    - os: linux
      command: echo
      args: ["test"]`

func TestPluginPackageWithoutSigning(t *testing.T) {
	// Create a test plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plugin.yaml file
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package options with sign=false
	o := &pluginPackageOptions{
		sign:        false, // Explicitly disable signing
		pluginPath:  pluginDir,
		destination: tempDir,
	}

	// Run the package command
	out := &bytes.Buffer{}
	err := o.run(out)

	// Should succeed without error
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that tarball was created with plugin name and version
	tarballPath := filepath.Join(tempDir, "test-plugin-1.0.0.tgz")
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		t.Error("tarball should exist when sign=false")
	}

	// Check that no .prov file was created
	provPath := tarballPath + ".prov"
	if _, err := os.Stat(provPath); !os.IsNotExist(err) {
		t.Error("provenance file should not exist when sign=false")
	}

	// Output should contain warning about skipping signing
	output := out.String()
	if !strings.Contains(output, "WARNING: Skipping plugin signing") {
		t.Error("should print warning when signing is skipped")
	}
	if !strings.Contains(output, "Successfully packaged") {
		t.Error("should print success message")
	}
}

func TestPluginPackageDefaultRequiresSigning(t *testing.T) {
	// Create a test plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plugin.yaml file
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package options with default sign=true and invalid keyring
	o := &pluginPackageOptions{
		sign:        true, // This is now the default
		keyring:     "/non/existent/keyring",
		pluginPath:  pluginDir,
		destination: tempDir,
	}

	// Run the package command
	out := &bytes.Buffer{}
	err := o.run(out)

	// Should fail because signing is required by default
	if err == nil {
		t.Error("expected error when signing fails with default settings")
	}

	// Check that no tarball was created
	tarballPath := filepath.Join(tempDir, "test-plugin.tgz")
	if _, err := os.Stat(tarballPath); !os.IsNotExist(err) {
		t.Error("tarball should not exist when signing fails")
	}
}

func TestPluginPackageSigningFailure(t *testing.T) {
	// Create a test plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plugin.yaml file
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package options with sign flag but invalid keyring
	o := &pluginPackageOptions{
		sign:        true,
		keyring:     "/non/existent/keyring", // This will cause signing to fail
		pluginPath:  pluginDir,
		destination: tempDir,
	}

	// Run the package command
	out := &bytes.Buffer{}
	err := o.run(out)

	// Should get an error
	if err == nil {
		t.Error("expected error when signing fails, got nil")
	}

	// Check that no tarball was created
	tarballPath := filepath.Join(tempDir, "test-plugin.tgz")
	if _, err := os.Stat(tarballPath); !os.IsNotExist(err) {
		t.Error("tarball should not exist when signing fails")
	}

	// Output should not contain success message
	if bytes.Contains(out.Bytes(), []byte("Successfully packaged")) {
		t.Error("should not print success message when signing fails")
	}
}
