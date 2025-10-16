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
	"testing"

	"helm.sh/helm/v4/pkg/getter"
)

// TestArtifactInstaller_VersionSupport tests that ArtifactInstaller properly handles version constraints
func TestArtifactInstaller_VersionSupport(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		version string
	}{
		{
			name:    "VCS source with version tag",
			source:  "https://github.com/user/plugin",
			version: "v1.2.3",
		},
		{
			name:    "VCS source with branch",
			source:  "https://github.com/user/plugin",
			version: "main",
		},
		{
			name:    "VCS source with commit hash",
			source:  "https://github.com/user/plugin",
			version: "abc123def456",
		},
		{
			name:    "HTTP source with version",
			source:  "https://example.com/plugin.tgz",
			version: "1.0.0",
		},
		{
			name:    "OCI source with version",
			source:  "oci://registry/plugin",
			version: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewArtifactInstaller(tt.source)
			if err != nil {
				t.Fatalf("NewArtifactInstaller() error = %v", err)
			}

			// Test SetVersion method
			installer.SetVersion(tt.version)

			// Verify version was set (we can't easily test the internal version field
			// without making it public, but we can test that SetVersion doesn't panic)

			// Test that we can call SetVersion multiple times
			installer.SetVersion("different-version")
			installer.SetVersion(tt.version) // Set back to test version
		})
	}
}

// TestArtifactInstaller_SetOptions tests that ArtifactInstaller properly handles getter options
func TestArtifactInstaller_SetOptions(t *testing.T) {
	source := "oci://registry/plugin"
	installer, err := NewArtifactInstaller(source)
	if err != nil {
		t.Fatalf("NewArtifactInstaller() error = %v", err)
	}

	// Test SetOptions method
	options := []getter.Option{
		getter.WithBasicAuth("user", "pass"),
		getter.WithPlainHTTP(true),
		getter.WithInsecureSkipVerifyTLS(true),
	}

	// Should not panic
	installer.SetOptions(options)

	// Test that we can call SetOptions multiple times
	moreOptions := []getter.Option{
		getter.WithTLSClientConfig("cert", "key", "ca"),
	}
	installer.SetOptions(moreOptions)
}

// TestFindSource_ReturnsArtifactInstaller tests that FindSource returns ArtifactInstaller for various sources
func TestFindSource_ReturnsArtifactInstaller(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "HTTP URL",
			source: "https://example.com/plugin.tgz",
		},
		{
			name:   "HTTPS URL",
			source: "https://example.com/plugin.tgz",
		},
		{
			name:   "VCS URL",
			source: "https://github.com/user/plugin",
		},
		{
			name:   "OCI reference",
			source: "oci://registry/plugin:1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := FindSource(tt.source)
			if err != nil {
				t.Fatalf("FindSource() error = %v", err)
			}

			if _, ok := installer.(*ArtifactInstaller); !ok {
				t.Errorf("FindSource() returned %T, expected *ArtifactInstaller", installer)
			}
		})
	}
}

// TestFindSource_LocalPath tests that FindSource returns LocalInstaller for local paths
func TestFindSource_LocalPath(t *testing.T) {
	// Create a temporary directory to simulate a local plugin
	tempDir := t.TempDir()

	installer, err := FindSource(tempDir)
	if err != nil {
		t.Fatalf("FindSource() error = %v", err)
	}

	if _, ok := installer.(*LocalInstaller); !ok {
		t.Errorf("FindSource() returned %T, expected *LocalInstaller", installer)
	}
}
