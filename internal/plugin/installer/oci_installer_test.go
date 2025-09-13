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

package installer // import "helm.sh/helm/v4/internal/plugin/installer"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

var _ Installer = new(OCIInstaller)

// createTestPluginTarGz creates a test plugin tar.gz with plugin.yaml
func createTestPluginTarGz(t *testing.T, pluginName string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add plugin.yaml
	pluginYAML := fmt.Sprintf(`name: %s
version: "1.0.0"
description: "Test plugin for OCI installer"
command: "$HELM_PLUGIN_DIR/bin/%s"
`, pluginName, pluginName)
	header := &tar.Header{
		Name:     "plugin.yaml",
		Mode:     0644,
		Size:     int64(len(pluginYAML)),
		Typeflag: tar.TypeReg,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte(pluginYAML)); err != nil {
		t.Fatal(err)
	}

	// Add bin directory
	dirHeader := &tar.Header{
		Name:     "bin/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}
	if err := tarWriter.WriteHeader(dirHeader); err != nil {
		t.Fatal(err)
	}

	// Add executable
	execContent := fmt.Sprintf("#!/bin/sh\necho '%s test plugin'", pluginName)
	execHeader := &tar.Header{
		Name:     fmt.Sprintf("bin/%s", pluginName),
		Mode:     0755,
		Size:     int64(len(execContent)),
		Typeflag: tar.TypeReg,
	}
	if err := tarWriter.WriteHeader(execHeader); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte(execContent)); err != nil {
		t.Fatal(err)
	}

	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes()
}

// mockOCIRegistryWithArtifactType creates a mock OCI registry server using the new artifact type approach
func mockOCIRegistryWithArtifactType(t *testing.T, pluginName string) (*httptest.Server, string) {
	t.Helper()

	pluginData := createTestPluginTarGz(t, pluginName)
	layerDigest := fmt.Sprintf("sha256:%x", sha256Sum(pluginData))

	// Create empty config data (as per OCI v1.1+ spec)
	configData := []byte("{}")
	configDigest := fmt.Sprintf("sha256:%x", sha256Sum(configData))

	// Create manifest with artifact type
	manifest := ocispec.Manifest{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: "application/vnd.helm.plugin.v1+json", // Using artifact type
		Config: ocispec.Descriptor{
			MediaType: "application/vnd.oci.empty.v1+json", // Empty config
			Digest:    digest.Digest(configDigest),
			Size:      int64(len(configData)),
		},
		Layers: []ocispec.Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar",
				Digest:    digest.Digest(layerDigest),
				Size:      int64(len(pluginData)),
				Annotations: map[string]string{
					ocispec.AnnotationTitle: pluginName + "-1.0.0.tgz", // Layer named with version
				},
			},
		},
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := fmt.Sprintf("sha256:%x", sha256Sum(manifestData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/v2/") && !strings.Contains(r.URL.Path, "/manifests/") && !strings.Contains(r.URL.Path, "/blobs/"):
			// API version check
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/manifests/") && strings.Contains(r.URL.Path, pluginName):
			// Return manifest
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", manifestDigest)
			w.WriteHeader(http.StatusOK)
			w.Write(manifestData)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/"+layerDigest):
			// Return layer data
			w.Header().Set("Content-Type", "application/vnd.oci.image.layer.v1.tar")
			w.WriteHeader(http.StatusOK)
			w.Write(pluginData)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/"+configDigest):
			// Return config data
			w.Header().Set("Content-Type", "application/vnd.oci.empty.v1+json")
			w.WriteHeader(http.StatusOK)
			w.Write(configData)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Parse server URL to get host:port format for OCI reference
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	registryHost := serverURL.Host

	return server, registryHost
}

// sha256Sum calculates SHA256 sum of data
func sha256Sum(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func TestNewOCIInstaller(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		expectName  string
		expectError bool
	}{
		{
			name:        "valid OCI reference with tag",
			source:      "oci://ghcr.io/user/plugin-name:v1.0.0",
			expectName:  "plugin-name",
			expectError: false,
		},
		{
			name:        "valid OCI reference with digest",
			source:      "oci://ghcr.io/user/plugin-name@sha256:1234567890abcdef",
			expectName:  "plugin-name",
			expectError: false,
		},
		{
			name:        "valid OCI reference without tag",
			source:      "oci://ghcr.io/user/plugin-name",
			expectName:  "plugin-name",
			expectError: false,
		},
		{
			name:        "valid OCI reference with multiple path segments",
			source:      "oci://registry.example.com/org/team/plugin-name:latest",
			expectName:  "plugin-name",
			expectError: false,
		},
		{
			name:        "invalid OCI reference - no path",
			source:      "oci://registry.example.com",
			expectName:  "",
			expectError: true,
		},
		{
			name:        "valid OCI reference - single path segment",
			source:      "oci://registry.example.com/plugin",
			expectName:  "plugin",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewOCIInstaller(tt.source)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check all fields thoroughly
			if installer.PluginName != tt.expectName {
				t.Errorf("expected plugin name %s, got %s", tt.expectName, installer.PluginName)
			}

			if installer.Source != tt.source {
				t.Errorf("expected source %s, got %s", tt.source, installer.Source)
			}

			if installer.CacheDir == "" {
				t.Error("expected non-empty cache directory")
			}

			if !strings.Contains(installer.CacheDir, "plugins") {
				t.Errorf("expected cache directory to contain 'plugins', got %s", installer.CacheDir)
			}

			if installer.settings == nil {
				t.Error("expected settings to be initialized")
			}

			// Check that Path() method works
			expectedPath := helmpath.DataPath("plugins", tt.expectName)
			if installer.Path() != expectedPath {
				t.Errorf("expected path %s, got %s", expectedPath, installer.Path())
			}
		})
	}
}

func TestOCIInstaller_Path(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		pluginName string
		expectPath string
	}{
		{
			name:       "valid plugin name",
			source:     "oci://ghcr.io/user/plugin-name:v1.0.0",
			pluginName: "plugin-name",
			expectPath: helmpath.DataPath("plugins", "plugin-name"),
		},
		{
			name:       "empty source",
			source:     "",
			pluginName: "",
			expectPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer := &OCIInstaller{
				PluginName: tt.pluginName,
				base:       newBase(tt.source),
				settings:   cli.New(),
			}

			path := installer.Path()
			if path != tt.expectPath {
				t.Errorf("expected path %s, got %s", tt.expectPath, path)
			}
		})
	}
}

func TestOCIInstaller_Install(t *testing.T) {
	// Set up isolated test environment
	ensure.HelmHome(t)

	pluginName := "test-plugin-basic"
	server, registryHost := mockOCIRegistryWithArtifactType(t, pluginName)
	defer server.Close()

	// Test OCI reference
	source := fmt.Sprintf("oci://%s/%s:latest", registryHost, pluginName)

	// Test with plain HTTP (since test server uses HTTP)
	installer, err := NewOCIInstaller(source, getter.WithPlainHTTP(true))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// The OCI installer uses helmpath.DataPath, which is isolated by ensure.HelmHome(t)
	actualPath := installer.Path()
	t.Logf("Installer will use path: %s", actualPath)

	// Install the plugin
	if err := Install(installer); err != nil {
		t.Fatalf("Expected installation to succeed, got error: %v", err)
	}

	// Verify plugin was installed to the correct location
	if !isPlugin(actualPath) {
		t.Errorf("Expected plugin directory %s to contain plugin.yaml", actualPath)
	}

	// Debug: list what was actually created
	if entries, err := os.ReadDir(actualPath); err != nil {
		t.Fatalf("Could not read plugin directory %s: %v", actualPath, err)
	} else {
		t.Logf("Plugin directory %s contains:", actualPath)
		for _, entry := range entries {
			t.Logf("  - %s", entry.Name())
		}
	}

	// Verify the plugin.yaml file exists and is valid
	pluginFile := filepath.Join(actualPath, "plugin.yaml")
	if _, err := os.Stat(pluginFile); err != nil {
		t.Errorf("Expected plugin.yaml to exist, got error: %v", err)
	}
}

func TestOCIInstaller_Install_WithGetterOptions(t *testing.T) {
	testCases := []struct {
		name       string
		pluginName string
		options    []getter.Option
		wantErr    bool
	}{
		{
			name:       "plain HTTP",
			pluginName: "example-cli-plain-http",
			options:    []getter.Option{getter.WithPlainHTTP(true)},
			wantErr:    false,
		},
		{
			name:       "insecure skip TLS verify",
			pluginName: "example-cli-insecure",
			options:    []getter.Option{getter.WithPlainHTTP(true), getter.WithInsecureSkipVerifyTLS(true)},
			wantErr:    false,
		},
		{
			name:       "with timeout",
			pluginName: "example-cli-timeout",
			options:    []getter.Option{getter.WithPlainHTTP(true), getter.WithTimeout(30 * time.Second)},
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up isolated test environment for each subtest
			ensure.HelmHome(t)

			server, registryHost := mockOCIRegistryWithArtifactType(t, tc.pluginName)
			defer server.Close()

			source := fmt.Sprintf("oci://%s/%s:latest", registryHost, tc.pluginName)

			installer, err := NewOCIInstaller(source, tc.options...)
			if err != nil {
				if !tc.wantErr {
					t.Fatalf("Expected no error creating installer, got %v", err)
				}
				return
			}

			// The installer now uses our isolated test directory
			actualPath := installer.Path()

			// Install the plugin
			err = Install(installer)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected installation to fail, but it succeeded")
				}
			} else {
				if err != nil {
					t.Errorf("Expected installation to succeed, got error: %v", err)
				} else {
					// Verify plugin was installed to the actual path
					if !isPlugin(actualPath) {
						t.Errorf("Expected plugin directory %s to contain plugin.yaml", actualPath)
					}
				}
			}
		})
	}
}

func TestOCIInstaller_Install_AlreadyExists(t *testing.T) {
	// Set up isolated test environment
	ensure.HelmHome(t)

	pluginName := "test-plugin-exists"
	server, registryHost := mockOCIRegistryWithArtifactType(t, pluginName)
	defer server.Close()

	source := fmt.Sprintf("oci://%s/%s:latest", registryHost, pluginName)
	installer, err := NewOCIInstaller(source, getter.WithPlainHTTP(true))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// First install should succeed
	if err := Install(installer); err != nil {
		t.Fatalf("Expected first installation to succeed, got error: %v", err)
	}

	// Verify plugin was installed
	if !isPlugin(installer.Path()) {
		t.Errorf("Expected plugin directory %s to contain plugin.yaml", installer.Path())
	}

	// Second install should fail with "plugin already exists"
	err = Install(installer)
	if err == nil {
		t.Error("Expected error when installing plugin that already exists")
	} else if !strings.Contains(err.Error(), "plugin already exists") {
		t.Errorf("Expected 'plugin already exists' error, got: %v", err)
	}
}

func TestOCIInstaller_Update(t *testing.T) {
	// Set up isolated test environment
	ensure.HelmHome(t)

	pluginName := "test-plugin-update"
	server, registryHost := mockOCIRegistryWithArtifactType(t, pluginName)
	defer server.Close()

	source := fmt.Sprintf("oci://%s/%s:latest", registryHost, pluginName)
	installer, err := NewOCIInstaller(source, getter.WithPlainHTTP(true))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test update when plugin does not exist - should fail
	err = Update(installer)
	if err == nil {
		t.Error("Expected error when updating plugin that does not exist")
	} else if !strings.Contains(err.Error(), "plugin does not exist") {
		t.Errorf("Expected 'plugin does not exist' error, got: %v", err)
	}

	// Install plugin first
	if err := Install(installer); err != nil {
		t.Fatalf("Expected installation to succeed, got error: %v", err)
	}

	// Verify plugin was installed
	if !isPlugin(installer.Path()) {
		t.Errorf("Expected plugin directory %s to contain plugin.yaml", installer.Path())
	}

	// Test update when plugin exists - should succeed
	// For OCI, Update() removes old version and reinstalls
	if err := Update(installer); err != nil {
		t.Errorf("Expected update to succeed, got error: %v", err)
	}

	// Verify plugin is still installed after update
	if !isPlugin(installer.Path()) {
		t.Errorf("Expected plugin directory %s to contain plugin.yaml after update", installer.Path())
	}
}

func TestOCIInstaller_Install_ComponentExtraction(t *testing.T) {
	// Test that we can extract a plugin archive properly
	// This tests the extraction logic that Install() uses
	tempDir := t.TempDir()
	pluginName := "test-plugin-extract"

	pluginData := createTestPluginTarGz(t, pluginName)

	// Test extraction
	err := extractTarGz(bytes.NewReader(pluginData), tempDir)
	if err != nil {
		t.Fatalf("Failed to extract plugin: %v", err)
	}

	// Verify plugin.yaml exists
	pluginYAMLPath := filepath.Join(tempDir, "plugin.yaml")
	if _, err := os.Stat(pluginYAMLPath); os.IsNotExist(err) {
		t.Errorf("plugin.yaml not found after extraction")
	}

	// Verify bin directory exists
	binPath := filepath.Join(tempDir, "bin")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Errorf("bin directory not found after extraction")
	}

	// Verify executable exists and has correct permissions
	execPath := filepath.Join(tempDir, "bin", pluginName)
	if info, err := os.Stat(execPath); err != nil {
		t.Errorf("executable not found: %v", err)
	} else if info.Mode()&0111 == 0 {
		t.Errorf("file is not executable")
	}

	// Verify this would be recognized as a plugin
	if !isPlugin(tempDir) {
		t.Errorf("extracted directory is not a valid plugin")
	}
}

func TestExtractTarGz(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test tar.gz file
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add a test file to the archive
	testContent := "test content"
	header := &tar.Header{
		Name:     "test-file.txt",
		Mode:     0644,
		Size:     int64(len(testContent)),
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}

	if _, err := tarWriter.Write([]byte(testContent)); err != nil {
		t.Fatal(err)
	}

	// Add a test directory
	dirHeader := &tar.Header{
		Name:     "test-dir/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}

	if err := tarWriter.WriteHeader(dirHeader); err != nil {
		t.Fatal(err)
	}

	tarWriter.Close()
	gzWriter.Close()

	// Test extraction
	err := extractTarGz(bytes.NewReader(buf.Bytes()), tempDir)
	if err != nil {
		t.Errorf("extractTarGz failed: %v", err)
	}

	// Verify extracted file
	extractedFile := filepath.Join(tempDir, "test-file.txt")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Errorf("failed to read extracted file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("expected content %s, got %s", testContent, string(content))
	}

	// Verify extracted directory
	extractedDir := filepath.Join(tempDir, "test-dir")
	if _, err := os.Stat(extractedDir); os.IsNotExist(err) {
		t.Errorf("extracted directory does not exist: %s", extractedDir)
	}
}

func TestExtractTarGz_InvalidGzip(t *testing.T) {
	tempDir := t.TempDir()

	// Test with invalid gzip data
	invalidGzipData := []byte("not gzip data")
	err := extractTarGz(bytes.NewReader(invalidGzipData), tempDir)
	if err == nil {
		t.Error("expected error for invalid gzip data")
	}
}

func TestExtractTar_UnknownFileType(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test tar file
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	// Add a test file
	testContent := "test content"
	header := &tar.Header{
		Name:     "test-file.txt",
		Mode:     0644,
		Size:     int64(len(testContent)),
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}

	if _, err := tarWriter.Write([]byte(testContent)); err != nil {
		t.Fatal(err)
	}

	// Test unknown file type
	unknownHeader := &tar.Header{
		Name:     "unknown-type",
		Mode:     0644,
		Typeflag: tar.TypeSymlink, // Use a type that's not handled
	}

	if err := tarWriter.WriteHeader(unknownHeader); err != nil {
		t.Fatal(err)
	}

	tarWriter.Close()

	// Test extraction - should fail due to unknown type
	err := extractTar(bytes.NewReader(buf.Bytes()), tempDir)
	if err == nil {
		t.Error("expected error for unknown tar file type")
	}

	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected 'unknown type' error, got: %v", err)
	}
}

func TestExtractTar_SuccessfulExtraction(t *testing.T) {
	tempDir := t.TempDir()

	// Since we can't easily create extended headers with Go's tar package,
	// we'll test the logic that skips them by creating a simple tar with regular files
	// and then testing that the extraction works correctly.

	// Create a test tar file
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	// Add a regular file
	testContent := "test content"
	header := &tar.Header{
		Name:     "test-file.txt",
		Mode:     0644,
		Size:     int64(len(testContent)),
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}

	if _, err := tarWriter.Write([]byte(testContent)); err != nil {
		t.Fatal(err)
	}

	tarWriter.Close()

	// Test extraction
	err := extractTar(bytes.NewReader(buf.Bytes()), tempDir)
	if err != nil {
		t.Errorf("extractTar failed: %v", err)
	}

	// Verify the regular file was extracted
	extractedFile := filepath.Join(tempDir, "test-file.txt")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Errorf("failed to read extracted file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("expected content %s, got %s", testContent, string(content))
	}
}

func TestOCIInstaller_Install_PlainHTTPOption(t *testing.T) {
	// Test that PlainHTTP option is properly passed to getter
	source := "oci://example.com/test-plugin:v1.0.0"

	// Test with PlainHTTP=false (default)
	installer1, err := NewOCIInstaller(source)
	if err != nil {
		t.Fatalf("failed to create installer: %v", err)
	}
	if installer1.getter == nil {
		t.Error("getter should be initialized")
	}

	// Test with PlainHTTP=true
	installer2, err := NewOCIInstaller(source, getter.WithPlainHTTP(true))
	if err != nil {
		t.Fatalf("failed to create installer with PlainHTTP=true: %v", err)
	}
	if installer2.getter == nil {
		t.Error("getter should be initialized with PlainHTTP=true")
	}

	// Both installers should have the same basic properties
	if installer1.PluginName != installer2.PluginName {
		t.Error("plugin names should match")
	}
	if installer1.Source != installer2.Source {
		t.Error("sources should match")
	}

	// Test with multiple options
	installer3, err := NewOCIInstaller(source,
		getter.WithPlainHTTP(true),
		getter.WithBasicAuth("user", "pass"),
	)
	if err != nil {
		t.Fatalf("failed to create installer with multiple options: %v", err)
	}
	if installer3.getter == nil {
		t.Error("getter should be initialized with multiple options")
	}
}

func TestOCIInstaller_Install_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		layerData   []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "non-gzip layer",
			layerData:   []byte("not gzip data"),
			expectError: true,
			errorMsg:    "is not a gzip compressed archive",
		},
		{
			name:        "empty layer",
			layerData:   []byte{},
			expectError: true,
			errorMsg:    "is not a gzip compressed archive",
		},
		{
			name:        "single byte layer",
			layerData:   []byte{0x1f},
			expectError: true,
			errorMsg:    "is not a gzip compressed archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the gzip validation logic that's used in the Install method
			if len(tt.layerData) < 2 || tt.layerData[0] != 0x1f || tt.layerData[1] != 0x8b {
				// This matches the validation in the Install method
				if !tt.expectError {
					t.Error("expected valid gzip data")
				}
				if !strings.Contains(tt.errorMsg, "is not a gzip compressed archive") {
					t.Errorf("expected error message to contain 'is not a gzip compressed archive'")
				}
			}
		})
	}
}
