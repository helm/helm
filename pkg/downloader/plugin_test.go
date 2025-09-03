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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/getter"
)

// MockHTTPClient implements HTTPClient for testing
type MockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
}

func (m *MockHTTPClient) Head(url string) (*http.Response, error) {
	if err, exists := m.errors[url]; exists {
		return nil, err
	}
	if resp, exists := m.responses[url]; exists {
		// Ensure response has required fields
		if resp.Body == nil {
			resp.Body = http.NoBody
		}
		return resp, nil
	}
	// Default: return 404
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}, nil
}

// MockHTTPGetter simulates HTTP operations for testing
type MockHTTPGetter struct {
	responses   map[string]*bytes.Buffer
	shouldError bool
}

func (m *MockHTTPGetter) Get(url string, _ ...getter.Option) (*bytes.Buffer, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock HTTP error")
	}

	if resp, exists := m.responses[url]; exists {
		return resp, nil
	}

	return nil, fmt.Errorf("URL not found: %s", url)
}

// MockVCSGetter simulates VCS operations for testing
type MockVCSGetter struct {
	testRepoPath string
	shouldError  bool
	response     *bytes.Buffer // For direct response control
}

func (m *MockVCSGetter) Get(_ string, _ ...getter.Option) (*bytes.Buffer, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock VCS error")
	}

	// If direct response is set, use it
	if m.response != nil {
		return m.response, nil
	}

	// Read plugin.yaml from test repository (for file-based tests)
	if m.testRepoPath != "" {
		pluginYamlPath := filepath.Join(m.testRepoPath, "plugin.yaml")
		content, err := os.ReadFile(pluginYamlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read plugin.yaml: %w", err)
		}
		return bytes.NewBuffer(content), nil
	}

	return bytes.NewBufferString("default mock VCS content"), nil
}

// RestrictToArtifactTypes implements getter.Restricted for MockVCSGetter
func (m *MockVCSGetter) RestrictToArtifactTypes() []string {
	return []string{"plugin"}
}

func TestPluginDownloader_VCSGetterRegistration(t *testing.T) {
	pd := &PluginDownloader{
		Cache:        &MockCache{data: make(map[[32]byte][]byte)},
		ContentCache: t.TempDir(),
	}

	// Initialize the downloader by accessing internal initialization logic
	// This mirrors what happens in DownloadTo but without the actual network operation
	getters := getter.Getters()

	// Add VCS getter specifically for plugins
	if _, err := getter.NewVCSGetter(getter.WithArtifactType("plugin")); err == nil {
		getters = append(getters, getter.Provider{
			Schemes: []string{"git", "git+http", "git+https", "git+ssh"},
			New: func(options ...getter.Option) (getter.Getter, error) {
				return getter.NewVCSGetter(append(options, getter.WithArtifactType("plugin"))...)
			},
		})
	}

	pd.downloader = &Downloader{
		Verify:       pd.Verify,
		Keyring:      pd.Keyring,
		Getters:      getters,
		Cache:        pd.Cache,
		ContentCache: pd.ContentCache,
	}

	assert.NotNil(t, pd.downloader)

	// Verify VCS getters are registered
	vcsSchemes := []string{"git", "git+http", "git+https", "git+ssh"}
	for _, scheme := range vcsSchemes {
		getter, err := pd.downloader.Getters.ByScheme(scheme)
		assert.NoError(t, err, "VCS getter should be registered for scheme %s", scheme)
		assert.NotNil(t, getter)
	}
}

func TestPluginDownloader_SmartHTTPDownloader_BothSchemes(t *testing.T) {
	pd := &PluginDownloader{
		Cache:        &MockCache{data: make(map[[32]byte][]byte)},
		ContentCache: t.TempDir(),
		Verify:       VerifyNever,
	}

	// Initialize the downloader to set up getters
	_, _, _ = pd.DownloadTo("https://example.com/plugin.tgz", "", t.TempDir())

	// Verify that both HTTP and HTTPS schemes are supported by the same provider
	// Since ByScheme creates new instances, we test by checking that both schemes work
	httpGetter, err := pd.downloader.Getters.ByScheme("http")
	assert.NoError(t, err, "HTTP getter should be registered")
	assert.NotNil(t, httpGetter)

	httpsGetter, err := pd.downloader.Getters.ByScheme("https")
	assert.NoError(t, err, "HTTPS getter should be registered")
	assert.NotNil(t, httpsGetter)

	// Both should implement the same artifact type restrictions
	if httpRestricted, ok := httpGetter.(getter.Restricted); ok {
		httpTypes := httpRestricted.RestrictToArtifactTypes()
		assert.Equal(t, []string{"plugin"}, httpTypes, "HTTP getter should be restricted to plugins")
	}

	if httpsRestricted, ok := httpsGetter.(getter.Restricted); ok {
		httpsTypes := httpsRestricted.RestrictToArtifactTypes()
		assert.Equal(t, []string{"plugin"}, httpsTypes, "HTTPS getter should be restricted to plugins")
	}
}

func TestPluginDownloader_VCSInstallationWorkflow(t *testing.T) {
	// This test demonstrates that plugin installation works through the new architecture
	// It mirrors the original VCS installer test patterns but uses the unified downloader
	ensure.HelmHome(t)

	// Create a test plugin directory structure (simulates a Git repository)
	tempDir := t.TempDir()
	testRepoPath := filepath.Join(tempDir, "test-plugin-repo")
	err := os.MkdirAll(testRepoPath, 0755)
	require.NoError(t, err)

	// Create plugin.yaml (what would be in the Git repository)
	pluginContent := `name: test-vcs-plugin
version: 1.2.3
usage: A VCS-installed test plugin
description: Plugin installed via VCS getter through unified downloader
command: echo "VCS plugin executed successfully"
`
	pluginYamlPath := filepath.Join(testRepoPath, "plugin.yaml")
	err = os.WriteFile(pluginYamlPath, []byte(pluginContent), 0644)
	require.NoError(t, err)

	// Mock the VCS getter to return our test plugin directory
	mockVCSGetter := &MockVCSGetter{
		testRepoPath: testRepoPath,
		shouldError:  false,
	}

	// Create plugin downloader with VCS transport
	pd := &PluginDownloader{
		Cache:        &MockCache{data: make(map[[32]byte][]byte)},
		ContentCache: t.TempDir(),
		Verify:       VerifyNever,
	}

	// Initialize downloader with our mock VCS getter
	getters := getter.Providers{
		{Schemes: []string{"git"}, New: func(...getter.Option) (getter.Getter, error) { return mockVCSGetter, nil }},
		{Schemes: []string{"git+https"}, New: func(...getter.Option) (getter.Getter, error) { return mockVCSGetter, nil }},
	}

	pd.downloader = &Downloader{
		Verify:       pd.Verify,
		Keyring:      pd.Keyring,
		Getters:      getters,
		Cache:        pd.Cache,
		ContentCache: pd.ContentCache,
	}

	// Test the download workflow
	destDir := t.TempDir()
	// VCS downloads don't involve provenance verification
	downloadedPath, _, err := pd.DownloadTo("git+https://github.com/example/test-plugin.git", "", destDir)

	// Verify the download succeeded
	assert.NoError(t, err, "Plugin download should succeed")
	assert.NotEmpty(t, downloadedPath, "Should return downloaded file path")

	// Verify content was written
	assert.FileExists(t, downloadedPath, "Downloaded plugin file should exist")

	// Verify content matches expectations
	downloadedContent, err := os.ReadFile(downloadedPath)
	assert.NoError(t, err)
	assert.Contains(t, string(downloadedContent), "name: test-vcs-plugin", "Downloaded content should contain plugin metadata")
	assert.Contains(t, string(downloadedContent), "version: 1.2.3", "Downloaded content should contain version")
}

func TestSmartHTTPDownloader_ArchiveDetection(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		contentType   string
		disposition   string
		expectError   bool
		expectArchive bool
	}{
		{"TarballContentType", "https://example.com/plugin.tgz", "application/gzip", "", false, true},
		{"OctetStreamContentType", "https://example.com/plugin.tar.gz", "application/octet-stream", "", false, true},
		{"AttachmentWithTgzExtension", "https://example.com/download", "text/html", "attachment; filename=plugin.tgz", false, true},
		{"HTMLContentType", "https://example.com/repo", "text/html", "", false, false},
		{"ProvFile", "https://example.com/plugin.tgz.prov", "text/plain", "", false, true}, // .prov files always use HTTP
		{"NetworkError", "https://example.com/error", "", "", true, false},
		{"NotFound", "https://example.com/notfound", "", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				responses: make(map[string]*http.Response),
				errors:    make(map[string]error),
			}

			if tt.expectError {
				mockClient.errors[tt.url] = fmt.Errorf("network error")
			} else {
				header := make(http.Header)
				if tt.contentType != "" {
					header.Set("Content-Type", tt.contentType)
				}
				if tt.disposition != "" {
					header.Set("Content-Disposition", tt.disposition)
				}
				mockClient.responses[tt.url] = &http.Response{
					StatusCode: http.StatusOK,
					Header:     header,
					Body:       http.NoBody,
				}
			}

			transport := &SmartHTTPDownloader{
				httpClient: mockClient,
			}

			isArchive := transport.servesArchiveContent(tt.url)
			assert.Equal(t, tt.expectArchive, isArchive, "Archive detection mismatch for %s", tt.url)
		})
	}
}

func TestSmartHTTPDownloader_ArtifactTypes(t *testing.T) {
	transport := &SmartHTTPDownloader{}

	// Should only support plugins
	supported := transport.RestrictToArtifactTypes()
	assert.Equal(t, []string{"plugin"}, supported, "SmartHTTPDownloader should only support plugins")
}

func TestSmartHTTPDownloader_DefaultHTTPClient(t *testing.T) {
	// Test that SmartHTTPDownloader uses default HTTP client when none injected
	transport := &SmartHTTPDownloader{}

	// This should not panic and should return false for non-existent URLs
	// (since default client will fail to connect)
	isArchive := transport.servesArchiveContent("https://nonexistent.example.com/file.tgz")
	assert.False(t, isArchive, "Non-existent URL should return false")
}

func TestSmartHTTPDownloader_ProvFileHandling(t *testing.T) {
	// Test that .prov files are always routed to HTTP getter, regardless of scheme
	// This prevents VCS getter from being attempted on provenance files

	mockVCSGetter := &MockVCSGetter{shouldError: true} // VCS should never be called
	mockHTTPGetter := &MockHTTPGetter{
		responses: map[string]*bytes.Buffer{
			"http://example.com/plugin.tgz.prov":                                    bytes.NewBufferString("provenance signature data"),
			"https://example.com/plugin.tgz.prov":                                   bytes.NewBufferString("provenance signature data"),
			"https://github.com/user/repo/releases/download/v1.0.0/plugin.tgz.prov": bytes.NewBufferString("github prov data"),
		},
	}

	transport := &SmartHTTPDownloader{
		vcsGetter:  mockVCSGetter,
		httpGetter: mockHTTPGetter,
		httpClient: nil, // .prov files bypass HTTP HEAD request entirely
	}

	tests := []struct {
		name string
		url  string
	}{
		{"HTTPProvFile", "http://example.com/plugin.tgz.prov"},
		{"HTTPSProvFile", "https://example.com/plugin.tgz.prov"},
		{"GitHubReleaseProvFile", "https://github.com/user/repo/releases/download/v1.0.0/plugin.tgz.prov"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should use HTTP getter directly without trying VCS
			result, err := transport.Get(tt.url)
			assert.NoError(t, err, "Prov file download should succeed")
			assert.NotNil(t, result, "Should return content")
			assert.Contains(t, result.String(), "prov")

			// Verify VCS getter was never called (would have returned error)
			// If VCS was called, we'd get an error since mockVCSGetter.shouldError = true
		})
	}
}

func TestSmartHTTPDownloader_ContentDetection(t *testing.T) {
	transport := &SmartHTTPDownloader{}

	tests := []struct {
		name    string
		content []byte
		isValid bool
	}{
		{"ValidGzipTarball", append([]byte{0x1f, 0x8b, 0x08, 0x00}, make([]byte, 100)...), true},
		{"HTMLContent", []byte("<!DOCTYPE html><html><head><title>Test</title></head><body>This is a test HTML page with sufficient length for the test.</body></html>"), false},
		{"SmallContent", []byte("small"), false},
		{"EmptyContent", []byte{}, false},
		{"BinaryContent", append([]byte{0x89, 0x50, 0x4E, 0x47}, make([]byte, 100)...), true}, // Conservative: assume valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := bytes.NewBuffer(tt.content)
			isValid := transport.isValidPluginContent(buffer)
			assert.Equal(t, tt.isValid, isValid, "Content detection mismatch for %s", tt.name)
		})
	}
}

func TestPluginDownloader_NewPluginDownloader(t *testing.T) {
	pd := NewPluginDownloader()
	assert.NotNil(t, pd, "Should create new plugin downloader")
	assert.Nil(t, pd.downloader, "Internal downloader should be nil until first use")
}

func TestPluginDownloader_DownloadToCache(t *testing.T) {
	tempDir := t.TempDir()

	pd := &PluginDownloader{
		Cache:        &MockCache{data: make(map[[32]byte][]byte)},
		ContentCache: tempDir,
		Verify:       VerifyNever,
	}

	// Mock a simple HTTP getter for this test
	mockHTTPGetter := &MockHTTPGetter{
		responses: map[string]*bytes.Buffer{
			"https://example.com/plugin.tgz": bytes.NewBufferString("mock plugin content"),
		},
	}

	// Set up minimal getter
	getters := getter.Providers{
		{Schemes: []string{"https"}, New: func(...getter.Option) (getter.Getter, error) { return mockHTTPGetter, nil }},
	}

	pd.downloader = &Downloader{
		Verify:       pd.Verify,
		Getters:      getters,
		Cache:        pd.Cache,
		ContentCache: pd.ContentCache,
	}

	// Test that DownloadToCache delegates to DownloadTo with ContentCache
	path, _, err := pd.DownloadToCache("https://example.com/plugin.tgz", "")
	assert.NoError(t, err, "DownloadToCache should succeed")
	assert.Contains(t, path, tempDir, "Should download to ContentCache directory")
}

func TestSmartHTTPDownloader_Get_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		headError      bool
		vcsResponse    *bytes.Buffer
		vcsError       error
		httpResponse   *bytes.Buffer
		httpError      error
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "VCSSucceedsAfterHEADFails",
			url:            "https://github.com/user/repo",
			headError:      true,
			vcsResponse:    bytes.NewBufferString("vcs plugin content"),
			vcsError:       nil,
			httpResponse:   nil,
			httpError:      fmt.Errorf("http error"),
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "HTTPFallbackWithValidContent",
			url:            "https://example.com/might-be-repo",
			headError:      true,
			vcsResponse:    nil,
			vcsError:       fmt.Errorf("vcs error"),
			httpResponse:   bytes.NewBuffer(append([]byte{0x1f, 0x8b}, make([]byte, 100)...)), // Valid gzip
			httpError:      nil,
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "HTTPFallbackWithInvalidContent",
			url:            "https://example.com/might-be-repo",
			headError:      true,
			vcsResponse:    nil,
			vcsError:       fmt.Errorf("vcs error"),
			httpResponse:   bytes.NewBufferString("<!DOCTYPE html><html>Not a plugin</html>"),
			httpError:      nil,
			expectError:    true,
			expectedErrMsg: "URL does not contain a valid plugin archive",
		},
		{
			name:           "BothFail",
			url:            "https://example.com/nowhere",
			headError:      true,
			vcsResponse:    nil,
			vcsError:       fmt.Errorf("vcs error"),
			httpResponse:   nil,
			httpError:      fmt.Errorf("http error"),
			expectError:    true,
			expectedErrMsg: "failed to download plugin: HTTP failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTPClient := &MockHTTPClient{
				responses: make(map[string]*http.Response),
				errors:    make(map[string]error),
			}

			if tt.headError {
				mockHTTPClient.errors[tt.url] = fmt.Errorf("head request failed")
			}

			mockVCSGetter := &MockVCSGetter{
				shouldError: tt.vcsError != nil,
				response:    tt.vcsResponse,
			}
			mockHTTPGetter := &MockHTTPGetter{
				responses:   make(map[string]*bytes.Buffer),
				shouldError: tt.httpError != nil,
			}

			if tt.httpResponse != nil {
				mockHTTPGetter.responses[tt.url] = tt.httpResponse
			}

			transport := &SmartHTTPDownloader{
				vcsGetter:  mockVCSGetter,
				httpGetter: mockHTTPGetter,
				httpClient: mockHTTPClient,
			}

			result, err := transport.Get(tt.url)

			if tt.expectError {
				assert.Error(t, err, "Should return error")
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Should not return error")
				assert.NotNil(t, result, "Should return result")
			}
		})
	}
}
