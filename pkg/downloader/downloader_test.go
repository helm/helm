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

package downloader

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/getter"
)

// MockGetter implements getter.Getter for testing.
type MockGetter struct {
	data           map[string][]byte
	err            error
	supportedTypes []string
}

func (m *MockGetter) Get(url string, _ ...getter.Option) (*bytes.Buffer, error) {
	if m.err != nil {
		return nil, m.err
	}

	data, exists := m.data[url]
	if !exists {
		data = []byte("mock data")
	}

	return bytes.NewBuffer(data), nil
}

// RestrictToArtifactTypes implements getter.Restricted.
func (m *MockGetter) RestrictToArtifactTypes() []string {
	if m.supportedTypes != nil {
		return m.supportedTypes
	}
	return nil
}

// MockCache implements Cache for testing.
type MockCache struct {
	data map[[sha256.Size]byte][]byte
}

func (c *MockCache) Get(key [sha256.Size]byte, cacheType string) (string, error) {
	if data, exists := c.data[key]; exists {
		pattern := "cache-*" + cacheType
		tmpfile, err := os.CreateTemp("", pattern)
		if err != nil {
			return "", err
		}
		defer tmpfile.Close()

		_, err = tmpfile.Write(data)
		return tmpfile.Name(), err
	}
	return "", os.ErrNotExist
}

func (c *MockCache) Put(key [sha256.Size]byte, data io.Reader, cacheType string) (string, error) {
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, data); err != nil {
		return "", err
	}
	c.data[key] = buf.Bytes()

	pattern := "cache-*" + cacheType
	tmpfile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	_, err = tmpfile.Write(buf.Bytes())
	return tmpfile.Name(), err
}

func TestDownloader_Download(t *testing.T) {
	// Create temporary directory for test
	tmpdir := t.TempDir()

	// Create mock getter
	mockGetter := &MockGetter{
		data: map[string][]byte{
			"http://example.com/chart.tgz": []byte("chart data"),
			"oci://example.com/plugin:v1":  []byte("plugin data"),
		},
	}

	// Create downloader with mock getters
	downloader := &Downloader{
		Getters: getter.Providers{
			{Schemes: []string{"http"}, New: func(...getter.Option) (getter.Getter, error) { return mockGetter, nil }},
			{Schemes: []string{"oci"}, New: func(...getter.Option) (getter.Getter, error) { return mockGetter, nil }},
		},
		Cache:        &MockCache{data: make(map[[sha256.Size]byte][]byte)},
		ContentCache: tmpdir,
		Verify:       VerifyNever, // Skip verification for this test
	}

	// Test chart download
	chartPath, verification, err := downloader.Download("http://example.com/chart.tgz", "", tmpdir, TypeChart)
	if err != nil {
		t.Errorf("Expected no error downloading chart, got %v", err)
	}
	if chartPath == "" {
		t.Error("Expected chart path, got empty string")
	}
	if verification == nil {
		t.Error("Expected verification object, got nil")
	}

	// Verify file was created
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		t.Errorf("Expected chart file to exist at %s", chartPath)
	}

	// Test plugin download
	pluginPath, _, err := downloader.Download("oci://example.com/plugin:v1", "", tmpdir, TypePlugin)
	if err != nil {
		t.Errorf("Expected no error downloading plugin, got %v", err)
	}
	if pluginPath == "" {
		t.Error("Expected plugin path, got empty string")
	}

	// Verify file was created
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Errorf("Expected plugin file to exist at %s", pluginPath)
	}
}

// Note: ChartDownloader and PluginDownloader compatibility tests moved to
// their respective test files (chart_test.go, plugin_test.go) since they
// now have proper implementations rather than wrappers

func TestDownloader_ResolveArtifactVersion_DirectURL(t *testing.T) {
	downloader := &Downloader{}

	// Test direct HTTP URL resolution
	hash, u, err := downloader.ResolveArtifactVersion("https://example.com/chart.tgz", "", TypeChart)
	if err != nil {
		t.Errorf("Expected no error for direct URL, got %v", err)
	}
	if hash != "" {
		t.Errorf("Expected empty hash for direct URL, got %s", hash)
	}
	if u == nil || u.String() != "https://example.com/chart.tgz" {
		t.Errorf("Expected URL https://example.com/chart.tgz, got %v", u)
	}

	// Test OCI URL resolution without registry client
	hash, u, err = downloader.ResolveArtifactVersion("oci://example.com/chart:v1.0.0", "", TypeChart)
	if err != nil {
		t.Errorf("Expected no error for OCI URL without client, got %v", err)
	}
	if hash != "" {
		t.Errorf("Expected empty hash for OCI URL without client, got %s", hash)
	}
	if u == nil || u.String() != "oci://example.com/chart:v1.0.0" {
		t.Errorf("Expected URL oci://example.com/chart:v1.0.0, got %v", u)
	}

	// Test plugin repository reference rejection
	_, _, err = downloader.ResolveArtifactVersion("repo/plugin", "1.0.0", TypePlugin)
	if err == nil {
		t.Error("Expected error for plugin repository reference")
	}
	if !strings.Contains(err.Error(), "repository-based plugin distribution is not supported") {
		t.Errorf("Expected plugin distribution error message, got: %v", err)
	}
}

func TestDownloader_ResolveArtifactVersion_RepositoryChart(t *testing.T) {
	// Create temporary directory for repository config
	tmpDir := t.TempDir()

	downloader := &Downloader{
		repositoryConfig: filepath.Join(tmpDir, "repositories.yaml"),
		repositoryCache:  filepath.Join(tmpDir, "cache"),
	}

	// Test chart repository reference when config doesn't exist
	_, _, err := downloader.ResolveArtifactVersion("stable/nginx", "1.0.0", TypeChart)

	// Should not panic and should handle missing config gracefully
	if err == nil {
		t.Error("Expected error when repository config doesn't exist")
	}

	// The error should be about missing config or repository not found, not a panic
	if strings.Contains(err.Error(), "panic") {
		t.Errorf("Got panic-related error: %v", err)
	}
}

func TestDownloader_ValidateArtifactTypeForScheme(t *testing.T) {
	// Mock VCS getter that only supports plugins
	vcsGetter := &MockGetter{data: make(map[string][]byte)}
	vcsGetter.supportedTypes = []string{"plugin"}

	// Mock general getter that supports all types
	generalGetter := &MockGetter{data: make(map[string][]byte)}

	getters := getter.Providers{
		{Schemes: []string{"git"}, New: func(...getter.Option) (getter.Getter, error) { return vcsGetter, nil }},
		{Schemes: []string{"git+http"}, New: func(...getter.Option) (getter.Getter, error) { return vcsGetter, nil }},
		{Schemes: []string{"git+https"}, New: func(...getter.Option) (getter.Getter, error) { return vcsGetter, nil }},
		{Schemes: []string{"git+ssh"}, New: func(...getter.Option) (getter.Getter, error) { return vcsGetter, nil }},
		{Schemes: []string{"http"}, New: func(...getter.Option) (getter.Getter, error) { return generalGetter, nil }},
		{Schemes: []string{"https"}, New: func(...getter.Option) (getter.Getter, error) { return generalGetter, nil }},
		{Schemes: []string{"oci"}, New: func(...getter.Option) (getter.Getter, error) { return generalGetter, nil }},
	}

	d := &Downloader{Getters: getters}

	tests := []struct {
		name          string
		scheme        string
		artifactType  Type
		expectError   bool
		errorContains string
	}{
		{
			name:         "AllowVCSForPlugins",
			scheme:       "git",
			artifactType: TypePlugin,
			expectError:  false,
		},
		{
			name:         "AllowGitHTTPForPlugins",
			scheme:       "git+http",
			artifactType: TypePlugin,
			expectError:  false,
		},
		{
			name:         "AllowGitHTTPSForPlugins",
			scheme:       "git+https",
			artifactType: TypePlugin,
			expectError:  false,
		},
		{
			name:         "AllowGitSSHForPlugins",
			scheme:       "git+ssh",
			artifactType: TypePlugin,
			expectError:  false,
		},
		{
			name:          "RejectVCSForCharts",
			scheme:        "git",
			artifactType:  TypeChart,
			expectError:   true,
			errorContains: "scheme git does not support artifact type chart",
		},
		{
			name:          "RejectGitHTTPForCharts",
			scheme:        "git+http",
			artifactType:  TypeChart,
			expectError:   true,
			errorContains: "scheme git+http does not support artifact type chart",
		},
		{
			name:          "RejectGitHTTPSForCharts",
			scheme:        "git+https",
			artifactType:  TypeChart,
			expectError:   true,
			errorContains: "scheme git+https does not support artifact type chart",
		},
		{
			name:          "RejectGitSSHForCharts",
			scheme:        "git+ssh",
			artifactType:  TypeChart,
			expectError:   true,
			errorContains: "scheme git+ssh does not support artifact type chart",
		},
		{
			name:         "AllowHTTPForCharts",
			scheme:       "http",
			artifactType: TypeChart,
			expectError:  false,
		},
		{
			name:         "AllowHTTPForPlugins",
			scheme:       "http",
			artifactType: TypePlugin,
			expectError:  false,
		},
		{
			name:         "AllowHTTPSForCharts",
			scheme:       "https",
			artifactType: TypeChart,
			expectError:  false,
		},
		{
			name:         "AllowHTTPSForPlugins",
			scheme:       "https",
			artifactType: TypePlugin,
			expectError:  false,
		},
		{
			name:         "AllowOCIForCharts",
			scheme:       "oci",
			artifactType: TypeChart,
			expectError:  false,
		},
		{
			name:         "AllowOCIForPlugins",
			scheme:       "oci",
			artifactType: TypePlugin,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.validateArtifactTypeForScheme(tt.scheme, tt.artifactType)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Cache tests moved to cache_test.go for better organization
