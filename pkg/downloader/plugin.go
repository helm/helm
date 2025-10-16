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

// Plugin-specific artifact downloading capabilities.
package downloader

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
)

// HTTPClient interface for HTTP operations, enables mocking
type HTTPClient interface {
	Head(url string) (*http.Response, error)
}

// PluginDownloader handles downloading plugins with plugin-specific features.
type PluginDownloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// RegistryClient for OCI plugin downloads
	RegistryClient *registry.Client

	// ContentCache is the location where Cache stores its files by default
	ContentCache string

	// Cache specifies the cache implementation to use.
	Cache Cache

	// Internal unified downloader
	downloader *Downloader
}

// DownloadTo downloads a plugin to the specified destination.
func (p *PluginDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	// Initialize the internal downloader if needed
	if p.downloader == nil {
		// Use built-in getters as base
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

		// Add smart HTTP getter for plugins that intelligently routes between VCS and HTTP getters.
		// Tries VCS first for repository URLs, then falls back to HTTP for direct file URLs.
		// Supports both HTTP and HTTPS (e.g., local instances, development servers, etc.)
		if httpGetter, err := getter.NewHTTPGetter(); err == nil {
			if vcsGetter, err := getter.NewVCSGetter(getter.WithArtifactType("plugin")); err == nil {
				smartGetter := &SmartHTTPDownloader{
					vcsGetter:  vcsGetter,
					httpGetter: httpGetter,
					httpClient: nil, // Use default HTTP client
				}
				getters = append(getters, getter.Provider{
					Schemes: []string{"http", "https"},
					New: func(_ ...getter.Option) (getter.Getter, error) {
						return smartGetter, nil
					},
				})
			}
		}

		p.downloader = &Downloader{
			Verify:       p.Verify,
			Keyring:      p.Keyring,
			Getters:      getters,
			Cache:        p.Cache,
			ContentCache: p.ContentCache,
		}
		if p.RegistryClient != nil {
			p.downloader.SetRegistryClient(p.RegistryClient)
		}
	}

	return p.downloader.Download(ref, version, dest, TypePlugin)
}

// DownloadToCache downloads a plugin to the cache.
func (p *PluginDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	return p.DownloadTo(ref, version, p.ContentCache)
}

// ResolvePluginVersion resolves a plugin reference to a URL.
// This delegates to the unified artifact downloader.
func (p *PluginDownloader) ResolvePluginVersion(ref, version string) (string, *url.URL, error) {
	// Initialize the internal downloader if needed
	if p.downloader == nil {
		p.downloader = &Downloader{}
		if p.RegistryClient != nil {
			p.downloader.SetRegistryClient(p.RegistryClient)
		}
	}

	return p.downloader.ResolveArtifactVersion(ref, version, TypePlugin)
}

// SmartHTTPDownloader tries VCS getter for repository URLs, falls back to HTTP for direct file URLs.
type SmartHTTPDownloader struct {
	vcsGetter  getter.Getter
	httpGetter getter.Getter
	httpClient HTTPClient // For dependency injection in tests
}

// DefaultHTTPClient implements HTTPClient using the standard http.Client
type DefaultHTTPClient struct{}

func (c *DefaultHTTPClient) Head(url string) (*http.Response, error) {
	return http.Head(url)
}

func (s *SmartHTTPDownloader) Get(url string, options ...getter.Option) (*bytes.Buffer, error) {
	// First, check if the URL serves a tarball via HTTP HEAD request
	if s.servesArchiveContent(url) {
		// URL serves archive content, use HTTP getter
		return s.httpGetter.Get(url, options...)
	}

	// URL doesn't serve archive content, try VCS getter for repository-style installation
	if vcsResult, vcsErr := s.vcsGetter.Get(url, options...); vcsErr == nil {
		return vcsResult, nil
	}

	// VCS failed, fall back to HTTP in case HEAD request was wrong or server doesn't support HEAD
	result, httpErr := s.httpGetter.Get(url, options...)
	if httpErr == nil && s.isValidPluginContent(result) {
		return result, nil
	}

	// Both approaches failed, return the most informative error
	if httpErr != nil {
		return nil, fmt.Errorf("failed to download plugin: HTTP failed (%v), VCS failed", httpErr)
	}

	return nil, fmt.Errorf("URL does not contain a valid plugin archive")
}

// servesArchiveContent checks if the URL serves archive content via HTTP HEAD request
func (s *SmartHTTPDownloader) servesArchiveContent(url string) bool {
	// .prov files are always served via HTTP, never VCS
	// This handles the case where the downloader makes separate calls for
	// both "plugin.tgz" and "plugin.tgz.prov" - the .prov file is not an archive
	if strings.HasSuffix(url, ".prov") {
		return true
	}

	// Use injected HTTP client or default client
	client := s.httpClient
	if client == nil {
		client = &DefaultHTTPClient{}
	}

	// Perform HEAD request to check Content-Type without downloading the content
	resp, err := client.Head(url)
	if err != nil {
		// HEAD request failed, we'll have to try other approaches
		return false
	}
	defer resp.Body.Close()

	// Check Content-Type header for archive formats
	contentType := resp.Header.Get("Content-Type")
	archiveTypes := []string{
		"application/gzip",
		"application/x-gzip",
		"application/x-tar",
		"application/x-compressed-tar",
		"application/octet-stream", // Generic binary, might be a tarball
	}

	for _, archiveType := range archiveTypes {
		if strings.Contains(contentType, archiveType) {
			return true
		}
	}

	// Check Content-Disposition for attachment with archive extensions
	disposition := resp.Header.Get("Content-Disposition")
	if strings.Contains(disposition, "attachment") {
		archiveExtensions := []string{".tgz", ".tar.gz", ".tar", ".zip"}
		for _, ext := range archiveExtensions {
			if strings.Contains(disposition, ext) {
				return true
			}
		}
	}

	return false
}

// isValidPluginContent checks if the content looks like a valid plugin tarball
func (s *SmartHTTPDownloader) isValidPluginContent(content *bytes.Buffer) bool {
	if content == nil || content.Len() < 100 {
		return false
	}

	// Check for tarball magic bytes
	data := content.Bytes()

	// Gzip magic bytes (0x1f, 0x8b) - most .tgz files
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return true
	}

	// Uncompressed tar magic "ustar" at offset 257
	if len(data) >= 262 && string(data[257:262]) == "ustar" {
		return true
	}

	// If it starts with HTML, it's definitely not a plugin
	htmlIndicators := []string{
		"<!DOCTYPE", "<!doctype", "<html", "<HTML", "<head", "<HEAD",
	}
	contentStr := string(data[:min(500, len(data))])
	for _, indicator := range htmlIndicators {
		if strings.Contains(contentStr, indicator) {
			return false
		}
	}

	// If we can't determine, assume it's valid (conservative approach)
	return true
}

// RestrictToArtifactTypes implements getter.Restricted.
func (s *SmartHTTPDownloader) RestrictToArtifactTypes() []string {
	return []string{"plugin"}
}

// NewPluginDownloader creates a new PluginDownloader.
func NewPluginDownloader() *PluginDownloader {
	return &PluginDownloader{}
}
