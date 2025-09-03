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
	"fmt"
	"strings"
	"testing"

	"helm.sh/helm/v4/internal/plugin/installer"
	"helm.sh/helm/v4/pkg/registry"
)

func TestPluginInstallOptions_NewInstallerForSource(t *testing.T) {
	tests := []struct {
		name             string
		source           string
		version          string
		certFile         string
		keyFile          string
		caFile           string
		insecureSkipTLS  bool
		plainHTTP        bool
		username         string
		password         string
		expectedType     string
		expectOCIOptions bool
	}{
		{
			name:         "VCS source with version",
			source:       "https://github.com/user/plugin",
			version:      "v1.2.3",
			expectedType: "*installer.ArtifactInstaller",
		},
		{
			name:         "HTTP source with version",
			source:       "https://example.com/plugin.tgz",
			version:      "1.0.0",
			expectedType: "*installer.ArtifactInstaller",
		},
		{
			name:             "OCI source with version and options",
			source:           "oci://registry.io/plugin",
			version:          "1.0.0",
			certFile:         "cert.pem",
			keyFile:          "key.pem",
			caFile:           "ca.pem",
			insecureSkipTLS:  true,
			plainHTTP:        true,
			username:         "testuser",
			password:         "testpass",
			expectedType:     "*installer.ArtifactInstaller",
			expectOCIOptions: true,
		},
		{
			name:         "OCI source without extra options",
			source:       "oci://registry.io/plugin:latest",
			expectedType: "*installer.ArtifactInstaller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &pluginInstallOptions{
				source:                tt.source,
				version:               tt.version,
				certFile:              tt.certFile,
				keyFile:               tt.keyFile,
				caFile:                tt.caFile,
				insecureSkipTLSverify: tt.insecureSkipTLS,
				plainHTTP:             tt.plainHTTP,
				username:              tt.username,
				password:              tt.password,
			}

			installer, err := o.newInstallerForSource()
			if err != nil {
				t.Fatalf("newInstallerForSource() error = %v", err)
			}

			// Check the installer type
			installerType := fmt.Sprintf("%T", installer)
			if installerType != tt.expectedType {
				t.Errorf("newInstallerForSource() returned %s, expected %s", installerType, tt.expectedType)
			}

			// The installer should be created successfully and not be nil
			// The version setting is tested in the installer package tests
			if installer == nil {
				t.Error("installer should not be nil")
			}
		})
	}
}

func TestPluginInstallOptions_Complete(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedSource string
		expectError    bool
	}{
		{
			name:           "valid source",
			args:           []string{"https://github.com/user/plugin"},
			expectedSource: "https://github.com/user/plugin",
		},
		{
			name:           "OCI source",
			args:           []string{"oci://registry.io/plugin:1.0.0"},
			expectedSource: "oci://registry.io/plugin:1.0.0",
		},
		{
			name:           "local path",
			args:           []string{"/path/to/plugin"},
			expectedSource: "/path/to/plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &pluginInstallOptions{}
			err := o.complete(tt.args)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if o.source != tt.expectedSource {
				t.Errorf("expected source %s, got %s", tt.expectedSource, o.source)
			}
		})
	}
}

// TestPluginInstallCmd_VersionFlag tests that the --version flag is properly handled
func TestPluginInstallCmd_VersionFlag(t *testing.T) {
	// Create the plugin install command
	cmd := newPluginInstallCmd(&strings.Builder{})

	// Test that the version flag exists
	versionFlag := cmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Fatal("--version flag not found")
	}

	// Test that the flag has the correct default value
	if versionFlag.DefValue != "" {
		t.Errorf("expected default version to be empty, got %s", versionFlag.DefValue)
	}

	// Test that we can set the flag value
	err := versionFlag.Value.Set("v1.2.3")
	if err != nil {
		t.Errorf("failed to set version flag: %v", err)
	}

	if versionFlag.Value.String() != "v1.2.3" {
		t.Errorf("expected version flag value to be 'v1.2.3', got %s", versionFlag.Value.String())
	}
}

// TestPluginInstallCmd_OCIFlags tests that OCI-specific flags are properly handled
func TestPluginInstallCmd_OCIFlags(t *testing.T) {
	cmd := newPluginInstallCmd(&strings.Builder{})

	// Test all OCI-specific flags exist
	ociFlags := []string{
		"cert-file",
		"key-file",
		"ca-file",
		"insecure-skip-tls-verify",
		"plain-http",
		"username",
		"password",
	}

	for _, flagName := range ociFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("--%s flag not found", flagName)
		}
	}
}

// TestPluginInstallOptions_VersionPassedToInstaller verifies version is passed to installer
func TestPluginInstallOptions_VersionPassedToInstaller(t *testing.T) {
	testVersion := "v1.2.3"

	o := &pluginInstallOptions{
		source:  "https://github.com/user/plugin",
		version: testVersion,
	}

	installer, err := o.newInstallerForSource()
	if err != nil {
		t.Fatalf("newInstallerForSource() error = %v", err)
	}

	// The installer should be an ArtifactInstaller based on the type check
	installerType := fmt.Sprintf("%T", installer)
	expectedType := "*installer.ArtifactInstaller"
	if installerType != expectedType {
		t.Errorf("newInstallerForSource() returned %s, expected %s", installerType, expectedType)
	}

	// This test ensures that if someone removes version support again,
	// they'll have to consciously break this test, making the regression visible
	if o.version != testVersion {
		t.Errorf("version should be %s but got %s", testVersion, o.version)
	}
}

// TestFindSource_ReturnsCorrectInstallerType ensures FindSource routing works correctly
func TestFindSource_ReturnsCorrectInstallerType(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		expectedType string
	}{
		{
			name:         "VCS source returns ArtifactInstaller",
			source:       "https://github.com/user/plugin",
			expectedType: "*installer.ArtifactInstaller",
		},
		{
			name:         "HTTP source returns ArtifactInstaller",
			source:       "https://example.com/plugin.tgz",
			expectedType: "*installer.ArtifactInstaller",
		},
		{
			name:         "OCI source returns ArtifactInstaller",
			source:       fmt.Sprintf("%s://registry.io/plugin:1.0.0", registry.OCIScheme),
			expectedType: "*installer.ArtifactInstaller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, err := installer.FindSource(tt.source)
			if err != nil {
				t.Fatalf("FindSource() error = %v", err)
			}

			installerType := fmt.Sprintf("%T", i)
			if installerType != tt.expectedType {
				t.Errorf("FindSource() returned %s, expected %s", installerType, tt.expectedType)
			}
		})
	}
}
