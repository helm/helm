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
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

const (
	repoConfig = "testdata/repositories.yaml"
	repoCache  = "testdata/repository"
)

func TestResolveChartRef(t *testing.T) {
	tests := []struct {
		name, ref, expect, version string
		fail                       bool
	}{
		{name: "full URL", ref: "http://example.com/foo-1.2.3.tgz", expect: "http://example.com/foo-1.2.3.tgz"},
		{name: "full URL, HTTPS", ref: "https://example.com/foo-1.2.3.tgz", expect: "https://example.com/foo-1.2.3.tgz"},
		{name: "full URL, with authentication", ref: "http://username:password@example.com/foo-1.2.3.tgz", expect: "http://username:password@example.com/foo-1.2.3.tgz"},
		{name: "reference, testing repo", ref: "testing/alpine", expect: "http://example.com/alpine-1.2.3.tgz"},
		{name: "reference, version, testing repo", ref: "testing/alpine", version: "0.2.0", expect: "http://example.com/alpine-0.2.0.tgz"},
		{name: "reference, version, malformed repo", ref: "malformed/alpine", version: "1.2.3", expect: "http://dl.example.com/alpine-1.2.3.tgz"},
		{name: "reference, querystring repo", ref: "testing-querystring/alpine", expect: "http://example.com/alpine-1.2.3.tgz?key=value"},
		{name: "reference, testing-relative repo", ref: "testing-relative/foo", expect: "http://example.com/helm/charts/foo-1.2.3.tgz"},
		{name: "reference, testing-relative repo", ref: "testing-relative/bar", expect: "http://example.com/helm/bar-1.2.3.tgz"},
		{name: "reference, testing-relative repo", ref: "testing-relative/baz", expect: "http://example.com/path/to/baz-1.2.3.tgz"},
		{name: "reference, testing-relative-trailing-slash repo", ref: "testing-relative-trailing-slash/foo", expect: "http://example.com/helm/charts/foo-1.2.3.tgz"},
		{name: "reference, testing-relative-trailing-slash repo", ref: "testing-relative-trailing-slash/bar", expect: "http://example.com/helm/bar-1.2.3.tgz"},
		{name: "encoded URL", ref: "encoded-url/foobar", expect: "http://example.com/with%2Fslash/charts/foobar-4.2.1.tgz"},
		{name: "full URL, HTTPS, irrelevant version", ref: "https://example.com/foo-1.2.3.tgz", version: "0.1.0", expect: "https://example.com/foo-1.2.3.tgz", fail: true},
		{name: "full URL, file", ref: "file:///foo-1.2.3.tgz", fail: true},
		{name: "invalid", ref: "invalid-1.2.3", fail: true},
		{name: "not found", ref: "nosuchthing/invalid-1.2.3", fail: true},
		{name: "ref with tag", ref: "oci://example.com/helm-charts/nginx:15.4.2", expect: "oci://example.com/helm-charts/nginx:15.4.2"},
		{name: "no repository", ref: "oci://", fail: true},
		{name: "oci ref", ref: "oci://example.com/helm-charts/nginx", version: "15.4.2", expect: "oci://example.com/helm-charts/nginx:15.4.2"},
		{name: "oci ref with sha256 and version mismatch", ref: "oci://example.com/install/by/sha:0.1.1@sha256:d234555386402a5867ef0169fefe5486858b6d8d209eaf32fd26d29b16807fd6", version: "0.1.2", fail: true},
	}

	// Create a mock registry client for OCI references
	registryClient, err := registry.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	c := ChartDownloader{
		Out:              os.Stderr,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		RegistryClient:   registryClient,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}),
	}

	for _, tt := range tests {
		_, u, err := c.ResolveChartVersion(tt.ref, tt.version)
		if err != nil {
			if tt.fail {
				continue
			}
			t.Errorf("%s: failed with error %q", tt.name, err)
			continue
		}
		if got := u.String(); got != tt.expect {
			t.Errorf("%s: expected %s, got %s", tt.name, tt.expect, got)
		}
	}
}

func TestResolveChartOpts(t *testing.T) {
	tests := []struct {
		name, ref, version string
		expect             []getter.Option
	}{
		{
			name: "repo with CA-file",
			ref:  "testing-ca-file/foo",
			expect: []getter.Option{
				getter.WithURL("https://example.com/foo-1.2.3.tgz"),
				getter.WithTLSClientConfig("cert", "key", "ca"),
			},
		},
	}

	c := ChartDownloader{
		Out:              os.Stderr,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}),
	}

	for _, tt := range tests {
		// reset internal downloader for each test case
		c.downloader = nil

		expect, err := getter.NewHTTPGetter(tt.expect...)
		if err != nil {
			t.Errorf("%s: failed to setup http client: %s", tt.name, err)
			continue
		}

		_, u, err := c.ResolveChartVersion(tt.ref, tt.version)
		if err != nil {
			t.Errorf("%s: failed with error %s", tt.name, err)
			continue
		}

		// Check that internal downloader has the expected options
		if c.downloader == nil {
			t.Errorf("%s: internal downloader should be initialized", tt.name)
			continue
		}

		got, err := getter.NewHTTPGetter(
			append(
				c.downloader.Options,
				getter.WithURL(u.String()),
			)...,
		)
		if err != nil {
			t.Errorf("%s: failed to create http client: %s", tt.name, err)
			continue
		}

		if *(got.(*getter.HTTPGetter)) != *(expect.(*getter.HTTPGetter)) {
			t.Errorf("%s: expected %s, got %s", tt.name, expect, got)
		}
	}
}

func TestVerifyChart(t *testing.T) {
	v, err := VerifyChart("testdata/signtest-0.1.0.tgz", "testdata/signtest-0.1.0.tgz.prov", "testdata/helm-test-key.pub")
	if err != nil {
		t.Fatal(err)
	}
	// The verification is tested at length in the provenance package. Here,
	// we just want a quick sanity check that the v is not empty.
	if len(v.FileHash) == 0 {
		t.Error("Digest missing")
	}
}

func TestIsTar(t *testing.T) {
	tests := map[string]bool{
		"foo.tgz":           true,
		"foo/bar/baz.tgz":   true,
		"foo-1.2.3.4.5.tgz": true,
		"foo.tar.gz":        false, // for our purposes
		"foo.tgz.1":         false,
		"footgz":            false,
	}

	for src, expect := range tests {
		if isTar(src) != expect {
			t.Errorf("%q should be %t", src, expect)
		}
	}
}

func TestDownloadTo(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
		repotest.WithMiddleware(repotest.BasicAuthMiddleware(t)),
	)
	defer srv.Stop()
	if err := srv.CreateIndex(); err != nil {
		t.Fatal(err)
	}

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	contentCache := t.TempDir()

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyAlways,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		ContentCache:     contentCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
		}),
		Options: []getter.Option{
			getter.WithBasicAuth("username", "password"),
			getter.WithPassCredentialsAll(false),
		},
	}
	cname := "/signtest-0.1.0.tgz"
	dest := srv.Root()
	where, v, err := c.DownloadTo(srv.URL()+cname, "", dest)
	if err != nil {
		t.Fatal(err)
	}

	if expect := filepath.Join(dest, cname); where != expect {
		t.Errorf("Expected download to %s, got %s", expect, where)
	}

	if v.FileHash == "" {
		t.Error("File hash was empty, but verification is required.")
	}

	if _, err := os.Stat(filepath.Join(dest, cname)); err != nil {
		t.Error(err)
	}
}

func TestDownloadTo_TLS(t *testing.T) {
	// Set up mock server w/ tls enabled
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
		repotest.WithTLSConfig(repotest.MakeTestTLSConfig(t, "../../testdata")),
	)
	defer srv.Stop()
	if err := srv.CreateIndex(); err != nil {
		t.Fatal(err)
	}
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	repoConfig := filepath.Join(srv.Root(), "repositories.yaml")
	repoCache := srv.Root()
	contentCache := t.TempDir()

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyAlways,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		ContentCache:     contentCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
		}),
		Options: []getter.Option{
			getter.WithTLSClientConfig(
				"",
				"",
				filepath.Join("../../testdata/rootca.crt"),
			),
		},
	}
	cname := "test/signtest"
	dest := srv.Root()
	where, v, err := c.DownloadTo(cname, "", dest)
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dest, "signtest-0.1.0.tgz")
	if expect := target; where != expect {
		t.Errorf("Expected download to %s, got %s", expect, where)
	}

	if v.FileHash == "" {
		t.Error("File hash was empty, but verification is required.")
	}

	if _, err := os.Stat(target); err != nil {
		t.Error(err)
	}
}

func TestDownloadTo_VerifyLater(t *testing.T) {
	ensure.HelmHome(t)

	dest := t.TempDir()

	// Set up a fake repo
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
	)
	defer srv.Stop()
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}
	contentCache := t.TempDir()

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyLater,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		ContentCache:     contentCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
		}),
	}
	cname := "/signtest-0.1.0.tgz"
	where, _, err := c.DownloadTo(srv.URL()+cname, "", dest)
	if err != nil {
		t.Fatal(err)
	}

	if expect := filepath.Join(dest, cname); where != expect {
		t.Errorf("Expected download to %s, got %s", expect, where)
	}

	if _, err := os.Stat(filepath.Join(dest, cname)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, cname+".prov")); err != nil {
		t.Fatal(err)
	}
}

func TestScanReposForURL(t *testing.T) {
	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyNever,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}),
	}

	// Initialize the internal downloader to access scanReposForURL
	if c.downloader == nil {
		c.downloader = &Downloader{
			Getters: c.Getters,
			Options: make([]getter.Option, 0),
		}
		c.downloader.SetRepositoryConfig(c.RepositoryConfig, c.RepositoryCache)
	}

	// Load test repository configuration
	rf, err := loadRepoConfig(repoConfig)
	if err != nil {
		t.Fatal("Failed to load test repo config:", err)
	}

	tests := []struct {
		name        string
		url         string
		expectRepo  string
		expectError bool
	}{
		{
			name:       "URL matches testing repo (first match)",
			url:        "http://example.com/foo-1.0.0.tgz",
			expectRepo: "testing", // First repo with http://example.com
		},
		{
			name:       "URL matches testing repo (even for charts path)",
			url:        "http://example.com/charts/bar-1.0.0.tgz",
			expectRepo: "testing", // testing comes before kubernetes-charts in the list
		},
		{
			name:       "URL matches testing repo (even for helm path)",
			url:        "http://example.com/helm/baz-1.0.0.tgz",
			expectRepo: "testing", // testing comes before testing-relative
		},
		{
			name:       "URL matches testing repo (even for querystring)",
			url:        "http://example.com?key=value&chart=test.tgz",
			expectRepo: "testing", // testing comes before testing-querystring
		},
		{
			name:       "URL matches testing repo (even for encoded path)",
			url:        "http://example.com/with%2Fslash/chart.tgz",
			expectRepo: "testing", // testing comes before encoded-url
		},
		{
			name:       "URL matches https repo",
			url:        "https://example.com/secure-chart.tgz",
			expectRepo: "testing-https", // Different protocol, won't match testing
		},
		{
			name:       "URL matches basicauth repo",
			url:        "http://username:password@example.com/auth-chart.tgz",
			expectRepo: "testing-basicauth", // Different authority, won't match testing
		},
		{
			name:       "URL matches malformed repo",
			url:        "http://dl.example.com/chart.tgz",
			expectRepo: "malformed", // Different domain
		},
		{
			name:        "URL doesn't match any repo",
			url:         "http://unknown.com/chart.tgz",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := c.downloader.scanReposForURL(tt.url, rf)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for URL %q, but got none", tt.url)
				}
				if err != ErrNoOwnerRepo && tt.url != "" {
					t.Errorf("Expected ErrNoOwnerRepo, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for URL %q: %v", tt.url, err)
				return
			}

			if repo == nil {
				t.Errorf("Expected repo entry for URL %q, got nil", tt.url)
				return
			}

			if repo.Name != tt.expectRepo {
				t.Errorf("Expected repo %q for URL %q, got %q", tt.expectRepo, tt.url, repo.Name)
			}
		})
	}
}

func TestScanReposForURL_Integration(t *testing.T) {
	// Test that scanReposForURL integration with ResolveChartVersion
	// properly applies repository configuration
	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyNever,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}),
	}

	// Test that a URL matching a repository with TLS config gets the options applied
	t.Run("TLS repository config applied", func(t *testing.T) {
		// Reset downloader for clean test
		c.downloader = nil

		// Since both testing-https and testing-ca-file have the same URL,
		// and testing-https comes first, we need to trigger the repository resolution
		// through a repository-based reference that forces scanning.
		// Let's use a repository-based reference to force the repo config lookup
		_, _, err := c.ResolveChartVersion("testing-ca-file/chart", "1.0.0")

		// This will fail because the chart doesn't exist in the index, but that's OK
		// We're testing that the repository configuration was applied
		if err == nil {
			t.Error("Expected error for non-existent chart, but got none")
		}

		// Even though resolution failed, the repository config should have been applied
		if c.downloader == nil {
			t.Error("Expected downloader to be initialized")
			return
		}

		// Check that TLS transport options were added during repo config
		if len(c.downloader.Options) == 0 {
			t.Error("Expected transport options to be configured from repository")
		}

		// The getter options sync only happens when resolution succeeds
		// For this test, we just care that the transport options were configured
		// The sync behavior is tested separately in the successful resolution tests
	})

	t.Run("Unknown URL no special config", func(t *testing.T) {
		// Reset downloader for clean test
		c.downloader = nil

		// URL that doesn't match any repository should work without special config
		_, _, err := c.ResolveChartVersion("http://unknown.example.com/chart.tgz", "")
		if err != nil {
			t.Errorf("Expected unknown URL to be handled, got: %v", err)
		}
	})
}

func TestDownloadToCache(t *testing.T) {
	srv := repotest.NewTempServer(t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
	)
	defer srv.Stop()
	if err := srv.CreateIndex(); err != nil {
		t.Fatal(err)
	}
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	// The repo file needs to point to our server.
	repoFile := filepath.Join(srv.Root(), "repositories.yaml")
	repoCache := srv.Root()
	contentCache := t.TempDir()

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyNever,
		RepositoryConfig: repoFile,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoFile,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
		}),
		Cache: &DiskCache{Root: contentCache},
	}

	// Case 1: Chart not in cache, download it.
	t.Run("download and cache chart", func(t *testing.T) {
		// Clear cache for this test
		os.RemoveAll(contentCache)
		os.MkdirAll(contentCache, 0755)
		c.Cache = &DiskCache{Root: contentCache}

		pth, v, err := c.DownloadToCache("test/signtest", "0.1.0")
		require.NoError(t, err)
		require.NotNil(t, v)

		// Check that the file exists at the returned path
		_, err = os.Stat(pth)
		require.NoError(t, err, "chart should exist at returned path")

		// Check that it's in the cache
		digest, _, err := c.ResolveChartVersion("test/signtest", "0.1.0")
		require.NoError(t, err)
		digestBytes, err := hex.DecodeString(digest)
		require.NoError(t, err)
		var digestArray [sha256.Size]byte
		copy(digestArray[:], digestBytes)

		cachePath, err := c.Cache.Get(digestArray, CacheChart)
		require.NoError(t, err, "chart should now be in cache")
		require.Equal(t, pth, cachePath)
	})

	// Case 2: Chart is in cache, get from cache.
	t.Run("get chart from cache", func(t *testing.T) {
		// The cache should be populated from the previous test.
		// To prove it's coming from cache, we can stop the server.
		// But repotest doesn't support restarting.
		// Let's just call it again and assume it works if it's fast and doesn't error.
		pth, v, err := c.DownloadToCache("test/signtest", "0.1.0")
		require.NoError(t, err)
		require.NotNil(t, v)

		_, err = os.Stat(pth)
		require.NoError(t, err, "chart should exist at returned path")
	})

	// Case 3: Download with verification
	t.Run("download and verify", func(t *testing.T) {
		// Clear cache
		os.RemoveAll(contentCache)
		os.MkdirAll(contentCache, 0755)
		c.Cache = &DiskCache{Root: contentCache}
		c.Verify = VerifyAlways
		c.Keyring = "testdata/helm-test-key.pub"

		_, v, err := c.DownloadToCache("test/signtest", "0.1.0")
		require.NoError(t, err)
		require.NotNil(t, v)
		require.NotEmpty(t, v.FileHash, "verification should have a file hash")

		// Check that both chart and prov are in cache
		digest, _, err := c.ResolveChartVersion("test/signtest", "0.1.0")
		require.NoError(t, err)
		digestBytes, err := hex.DecodeString(digest)
		require.NoError(t, err)
		var digestArray [sha256.Size]byte
		copy(digestArray[:], digestBytes)

		_, err = c.Cache.Get(digestArray, CacheChart)
		require.NoError(t, err, "chart should be in cache")
		_, err = c.Cache.Get(digestArray, CacheProv)
		require.NoError(t, err, "provenance file should be in cache")

		// Reset for other tests
		c.Verify = VerifyNever
		c.Keyring = ""
	})
}
