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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	godigest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
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
	require.NoError(t, err)

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
		t.Run(tt.name, func(t *testing.T) {
			_, u, err := c.ResolveChartVersion(tt.ref, tt.version)
			if err != nil {
				require.True(t, tt.fail)
			} else {
				got := u.String()
				assert.Equalf(t, tt.expect, got, "%s: expected %s, got %s", tt.name, tt.expect, got)
			}
		})
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

	// snapshot options
	snapshotOpts := c.Options

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset chart downloader options for each test case
			c.Options = snapshotOpts

			expect, err := getter.NewHTTPGetter(tt.expect...)
			require.NoError(t, err, "failed to setup http client")

			_, u, err := c.ResolveChartVersion(tt.ref, tt.version)
			require.NoError(t, err, "failed with error")

			got, err := getter.NewHTTPGetter(
				append(
					c.Options,
					getter.WithURL(u.String()),
				)...,
			)
			require.NoError(t, err, "failed to create http client")
			assert.Equal(t, expect, got)
		})
	}
}

func TestVerifyChart(t *testing.T) {
	v, err := VerifyChart("testdata/signtest-0.1.0.tgz", "testdata/signtest-0.1.0.tgz.prov", "testdata/helm-test-key.pub")
	require.NoError(t, err)
	// The verification is tested at length in the provenance package. Here,
	// we just want a quick sanity check that the v is not empty.
	assert.NotEmpty(t, v.FileHash, "Digest missing")
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
		assert.Equal(t, expect, isTar(src), "%q should be %t", src, expect)
	}
}

func TestDownloadTo(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
		repotest.WithMiddleware(repotest.BasicAuthMiddleware(t)),
	)
	defer srv.Stop()
	require.NoError(t, srv.CreateIndex())
	require.NoError(t, srv.LinkIndices())

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
	require.NoError(t, err)

	expect := filepath.Join(dest, cname)
	assert.Equalf(t, expect, where, "Expected download to %s, got %s", expect, where)
	assert.NotEmpty(t, v.FileHash, "File hash was empty, but verification is required.")

	_, err = os.Stat(filepath.Join(dest, cname))
	assert.NoError(t, err)
}

func TestDownloadTo_TLS(t *testing.T) {
	// Set up mock server w/ tls enabled
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
		repotest.WithTLSConfig(repotest.MakeTestTLSConfig(t, "../../testdata")),
	)
	defer srv.Stop()
	require.NoError(t, srv.CreateIndex())
	require.NoError(t, srv.LinkIndices())

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
				filepath.FromSlash("../../testdata/rootca.crt"),
			),
		},
	}
	cname := "test/signtest"
	dest := srv.Root()
	where, v, err := c.DownloadTo(cname, "", dest)
	require.NoError(t, err)

	target := filepath.Join(dest, "signtest-0.1.0.tgz")
	expect := target
	assert.Equalf(t, expect, where, "Expected download to %s, got %s", expect, where)
	assert.NotEmpty(t, v.FileHash, "File hash was empty, but verification is required.")

	_, err = os.Stat(target)
	assert.NoError(t, err)
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
	require.NoError(t, srv.LinkIndices())
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
	require.NoError(t, err)

	expect := filepath.Join(dest, cname)
	assert.Equalf(t, expect, where, "Expected download to %s, got %s", expect, where)

	_, err = os.Stat(filepath.Join(dest, cname))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dest, cname+".prov"))
	require.NoError(t, err)
}

func TestScanReposForURL(t *testing.T) {
	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyLater,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}),
	}

	u := "http://example.com/alpine-0.2.0.tgz"
	rf, err := repo.LoadFile(repoConfig)
	require.NoError(t, err)

	entry, err := c.scanReposForURL(u, rf)
	require.NoError(t, err)

	assert.Equal(t, "testing", entry.Name, "Unexpected repo %q for URL %q", entry.Name, u)

	// A lookup failure should produce an ErrNoOwnerRepo
	u = "https://no.such.repo/foo/bar-1.23.4.tgz"
	_, err = c.scanReposForURL(u, rf)
	require.ErrorIs(t, err, ErrNoOwnerRepo)
}

func TestDownloadToCache(t *testing.T) {
	srv := repotest.NewTempServer(t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
	)
	defer srv.Stop()
	require.NoError(t, srv.CreateIndex())
	require.NoError(t, srv.LinkIndices())

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
		os.MkdirAll(contentCache, 0o755)
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
		os.MkdirAll(contentCache, 0o755)
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

func TestDownloadToCachePassesOptionsToProvenance(t *testing.T) {
	chartData, err := os.ReadFile("testdata/signtest-0.1.0.tgz")
	require.NoError(t, err)
	provData, err := os.ReadFile("testdata/signtest-0.1.0.tgz.prov")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "username" || password != "password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/signtest-0.1.0.tgz":
			_, _ = w.Write(chartData)
		case "/signtest-0.1.0.tgz.prov":
			_, _ = w.Write(provData)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	contentCache := t.TempDir()
	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyLater,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
		}),
		Options: []getter.Option{
			getter.WithBasicAuth("username", "password"),
		},
		Cache: &DiskCache{Root: contentCache},
	}

	_, _, err = c.DownloadToCache(srv.URL+"/signtest-0.1.0.tgz", "")
	require.NoError(t, err)

	digest := sha256.Sum256(chartData)
	_, err = c.Cache.Get(digest, CacheProv)
	require.NoError(t, err, "provenance file should be in cache")
}

func TestStripDigestAlgorithm(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"sha256 prefixed digest": {
			input:    "sha256:aef46c66a7f2d5a12a7e3f54a64790daf5c9a9e66af3f46955efdaa6c900341d",
			expected: "aef46c66a7f2d5a12a7e3f54a64790daf5c9a9e66af3f46955efdaa6c900341d",
		},
		"sha512 prefixed digest": {
			input:    "sha512:abcdef1234567890",
			expected: "abcdef1234567890",
		},
		"plain hex digest without prefix": {
			input:    "aef46c66a7f2d5a12a7e3f54a64790daf5c9a9e66af3f46955efdaa6c900341d",
			expected: "aef46c66a7f2d5a12a7e3f54a64790daf5c9a9e66af3f46955efdaa6c900341d",
		},
		"empty string": {
			input:    "",
			expected: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := stripDigestAlgorithm(tt.input)
			assert.Equalf(t, tt.expected, result, "stripDigestAlgorithm(%q) = %q, want %q", tt.input, result, tt.expected)
		})
	}
}

// writeRepoCacheIndex publishes the generated index under the name the repo
// cache looks for. repotest.Server.LinkIndices symlinks instead of copying,
// which needs a privilege Windows does not grant by default.
func writeRepoCacheIndex(t *testing.T, root string) {
	t.Helper()
	idx, err := os.ReadFile(filepath.Join(root, "index.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "test-index.yaml"), idx, 0o644))
}

// tamperedChartServer serves a repository whose index records the real digest
// of signtest-0.1.0.tgz while the archive itself has been replaced. It returns
// the server, a downloader pointed at it, and the bytes now being served.
func tamperedChartServer(t *testing.T, contentCache string) (*repotest.Server, *ChartDownloader, []byte) {
	t.Helper()
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/*.tgz*"))
	t.Cleanup(srv.Stop)
	require.NoError(t, srv.CreateIndex())
	writeRepoCacheIndex(t, srv.Root())

	served := filepath.Join(srv.Root(), "signtest-0.1.0.tgz")
	original, err := os.ReadFile(served)
	require.NoError(t, err)
	tampered := append(append([]byte(nil), original...), []byte("appended by a rewritten mirror")...)
	require.NoError(t, os.WriteFile(served, tampered, 0o644))

	repoFile := filepath.Join(srv.Root(), "repositories.yaml")
	c := &ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyNever,
		RepositoryConfig: repoFile,
		RepositoryCache:  srv.Root(),
		ContentCache:     contentCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoFile,
			RepositoryCache:  srv.Root(),
			ContentCache:     contentCache,
		}),
		Cache: &DiskCache{Root: contentCache},
	}
	return srv, c, tampered
}

func TestDownloadTo_RejectsChartNotMatchingIndexDigest(t *testing.T) {
	contentCache := t.TempDir()
	dest := t.TempDir()
	_, c, _ := tamperedChartServer(t, contentCache)

	_, _, err := c.DownloadTo("test/signtest", "0.1.0", dest)
	require.Error(t, err, "a chart that does not match the index digest must not be accepted")
	assert.Contains(t, err.Error(), "does not match the digest recorded for it in the repository index")

	// Nothing may be left behind in dest for a later step to pick up.
	entries, err := os.ReadDir(dest)
	require.NoError(t, err)
	assert.Empty(t, entries, "rejected chart must not be written to the destination")
}

func TestDownloadToCache_RejectsChartNotMatchingIndexDigest(t *testing.T) {
	contentCache := t.TempDir()
	_, c, _ := tamperedChartServer(t, contentCache)

	digestString, _, err := c.ResolveChartVersion("test/signtest", "0.1.0")
	require.NoError(t, err)
	digestBytes, err := hex.DecodeString(stripDigestAlgorithm(digestString))
	require.NoError(t, err)
	var want [sha256.Size]byte
	copy(want[:], digestBytes)

	_, _, err = c.DownloadToCache("test/signtest", "0.1.0")
	require.Error(t, err, "a chart that does not match the index digest must not be accepted")
	assert.Contains(t, err.Error(), "does not match the digest recorded for it in the repository index")

	// The rejected bytes must not have been filed in the content cache under
	// the digest they failed to match.
	_, err = c.Cache.Get(want, CacheChart)
	assert.Error(t, err, "rejected chart must not be written to the content cache")
}

func TestDownloadToCache_DiscardsCacheEntryNotMatchingItsDigest(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/*.tgz*"))
	defer srv.Stop()
	require.NoError(t, srv.CreateIndex())
	writeRepoCacheIndex(t, srv.Root())

	repoFile := filepath.Join(srv.Root(), "repositories.yaml")
	contentCache := t.TempDir()
	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyNever,
		RepositoryConfig: repoFile,
		RepositoryCache:  srv.Root(),
		ContentCache:     contentCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoFile,
			RepositoryCache:  srv.Root(),
			ContentCache:     contentCache,
		}),
		Cache: &DiskCache{Root: contentCache},
	}

	digestString, _, err := c.ResolveChartVersion("test/signtest", "0.1.0")
	require.NoError(t, err)
	digestBytes, err := hex.DecodeString(stripDigestAlgorithm(digestString))
	require.NoError(t, err)
	var want [sha256.Size]byte
	copy(want[:], digestBytes)

	// Poison the content cache the way an older Helm could have: content that
	// does not hash to the key it is stored under.
	poison := []byte("not the chart this digest names")
	_, err = c.Cache.Put(want, bytes.NewBuffer(poison), CacheChart)
	require.NoError(t, err)

	pth, _, err := c.DownloadToCache("test/signtest", "0.1.0")
	require.NoError(t, err, "a bad cache entry should be replaced by a fresh download, not returned")

	got, err := os.ReadFile(pth)
	require.NoError(t, err)
	assert.NotEqual(t, poison, got, "poisoned cache entry must not be served")
	assert.Equal(t, want, sha256.Sum256(got), "served chart must hash to the index digest")
}

func TestIndexDigestVerificationScope(t *testing.T) {
	good := []byte("chart bytes")
	want := sha256.Sum256(good)

	t.Run("mismatch is rejected", func(t *testing.T) {
		err := verifyIndexDigest("ref", hex.EncodeToString(want[:]), want, []byte("other bytes"))
		assert.Error(t, err)
	})
	t.Run("match is accepted", func(t *testing.T) {
		err := verifyIndexDigest("ref", hex.EncodeToString(want[:]), want, good)
		assert.NoError(t, err)
	})
	t.Run("no index digest is a no-op", func(t *testing.T) {
		// A chart referenced by a bare URL has no index entry to check against.
		err := verifyIndexDigest("ref", "", [sha256.Size]byte{}, []byte("anything"))
		assert.NoError(t, err)
	})
}

// ociChartDownloader pushes testdata/signtest-0.1.0.tgz to an in-process
// registry and returns a downloader wired to it, the chart reference pinned to
// the pushed manifest digest, the push result and the registry.
func ociChartDownloader(t *testing.T, contentCache string) (*ChartDownloader, string, *registry.PushResult, *repotest.OCIServer) {
	t.Helper()
	dir := t.TempDir()
	srv, err := repotest.NewOCIServer(t, dir)
	require.NoError(t, err)
	go srv.ListenAndServe()
	dialer := &net.Dialer{Timeout: time.Second}
	require.Eventually(t, func() bool {
		conn, err := dialer.DialContext(t.Context(), "tcp", srv.RegistryURL)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 30*time.Second, 20*time.Millisecond)

	client, err := registry.NewClient(
		registry.ClientOptCredentialsFile(filepath.Join(dir, "config.json")),
		registry.ClientOptPlainHTTP(),
	)
	require.NoError(t, err)
	require.NoError(t, client.Login(srv.RegistryURL,
		registry.LoginOptBasicAuth(srv.TestUsername, srv.TestPassword),
		registry.LoginOptInsecure(true),
		registry.LoginOptPlainText(true)))

	archive, err := os.ReadFile("testdata/signtest-0.1.0.tgz")
	require.NoError(t, err)
	pushed, err := client.Push(archive, srv.RegistryURL+"/u/ocitestuser/signtest:0.1.0")
	require.NoError(t, err)

	settings := &cli.EnvSettings{ContentCache: contentCache}
	c := &ChartDownloader{
		Out:            os.Stderr,
		Verify:         VerifyNever,
		ContentCache:   contentCache,
		Getters:        getter.All(settings),
		Options:        []getter.Option{getter.WithRegistryClient(client)},
		RegistryClient: client,
		Cache:          &DiskCache{Root: contentCache},
	}
	ref := "oci://" + srv.RegistryURL + "/u/ocitestuser/signtest@" + pushed.Manifest.Digest
	return c, ref, pushed, srv
}

func digestKey(t *testing.T, d string) [sha256.Size]byte {
	t.Helper()
	b, err := hex.DecodeString(stripDigestAlgorithm(d))
	require.NoError(t, err)
	require.Len(t, b, sha256.Size)
	var k [sha256.Size]byte
	copy(k[:], b)
	return k
}

func TestDownloadToCache_OCIKeyedByChartLayerDigest(t *testing.T) {
	contentCache := t.TempDir()
	c, ref, pushed, _ := ociChartDownloader(t, contentCache)
	layer := digestKey(t, pushed.Chart.Digest)

	pth, _, err := c.DownloadToCache(ref, "0.1.0")
	require.NoError(t, err)
	got, err := os.ReadFile(pth)
	require.NoError(t, err)
	assert.Equal(t, layer, sha256.Sum256(got), "cached chart must hash to its chart layer digest")

	// The entry must be filed under the chart layer digest, where it can be
	// checked, and not under the manifest digest, where it could not.
	layerPath, err := c.Cache.Get(layer, CacheChart)
	require.NoError(t, err)
	assert.Equal(t, layerPath, pth)
	_, err = c.Cache.Get(digestKey(t, pushed.Manifest.Digest), CacheChart)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDownloadToCache_OCIDiscardsCacheEntryNotMatchingItsDigest(t *testing.T) {
	contentCache := t.TempDir()
	c, ref, pushed, _ := ociChartDownloader(t, contentCache)
	layer := digestKey(t, pushed.Chart.Digest)

	poison := []byte("not the chart this digest names")
	_, err := c.Cache.Put(layer, bytes.NewBuffer(poison), CacheChart)
	require.NoError(t, err)

	pth, _, err := c.DownloadToCache(ref, "0.1.0")
	require.NoError(t, err, "a bad cache entry should be replaced by a fresh download, not returned")
	got, err := os.ReadFile(pth)
	require.NoError(t, err)
	assert.Equal(t, layer, sha256.Sum256(got), "served chart must hash to its chart layer digest")
}

func TestDownloadTo_OCIIgnoresCacheEntryNotMatchingItsDigest(t *testing.T) {
	contentCache := t.TempDir()
	dest := t.TempDir()
	c, ref, pushed, _ := ociChartDownloader(t, contentCache)
	layer := digestKey(t, pushed.Chart.Digest)

	// Before this change an OCI chart was cached under its manifest digest.
	// Neither an entry like that nor a bad one under the layer digest may be
	// served in place of the chart.
	poison := []byte("not the chart this digest names")
	_, err := c.Cache.Put(digestKey(t, pushed.Manifest.Digest), bytes.NewBuffer(poison), CacheChart)
	require.NoError(t, err)
	_, err = c.Cache.Put(layer, bytes.NewBuffer(poison), CacheChart)
	require.NoError(t, err)

	saved, _, err := c.DownloadTo(ref, "0.1.0", dest)
	require.NoError(t, err)
	got, err := os.ReadFile(saved)
	require.NoError(t, err)
	assert.Equal(t, layer, sha256.Sum256(got), "saved chart must hash to its chart layer digest")
}

func TestDownloadTo_OCIDigestOnlyRef(t *testing.T) {
	c, ref, pushed, _ := ociChartDownloader(t, t.TempDir())

	// With no version the registry client resolves no digest for the ref, so
	// the cache is not consulted and the chart is downloaded directly.
	saved, _, err := c.DownloadTo(ref, "", t.TempDir())
	require.NoError(t, err)
	got, err := os.ReadFile(saved)
	require.NoError(t, err)
	assert.Equal(t, digestKey(t, pushed.Chart.Digest), sha256.Sum256(got))
}

func TestDownloadToCache_OCIImageIndexRoot(t *testing.T) {
	contentCache := t.TempDir()
	c, _, pushed, srv := ociChartDownloader(t, contentCache)
	layer := digestKey(t, pushed.Chart.Digest)

	// Wrap the chart manifest in an image index and move the tag onto it.
	repo, err := remote.NewRepository(srv.RegistryURL + "/u/ocitestuser/signtest")
	require.NoError(t, err)
	repo.PlainHTTP = true
	repo.Client = &auth.Client{Credential: auth.StaticCredential(srv.RegistryURL, auth.Credential{
		Username: srv.TestUsername,
		Password: srv.TestPassword,
	})}
	ctx := context.Background()
	manifest, err := repo.Resolve(ctx, "0.1.0")
	require.NoError(t, err)
	index, err := json.Marshal(ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{manifest},
	})
	require.NoError(t, err)
	indexDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
		Digest:    godigest.FromBytes(index),
		Size:      int64(len(index)),
	}
	require.NoError(t, repo.PushReference(ctx, indexDesc, bytes.NewReader(index), "0.1.0"))
	ref := "oci://" + srv.RegistryURL + "/u/ocitestuser/signtest@" + indexDesc.Digest.String()

	pth, _, err := c.DownloadToCache(ref, "0.1.0")
	require.NoError(t, err, "a chart behind an image index must still download")
	got, err := os.ReadFile(pth)
	require.NoError(t, err)
	assert.Equal(t, layer, sha256.Sum256(got))
	_, err = c.Cache.Get(digestKey(t, indexDesc.Digest.String()), CacheChart)
	require.ErrorIs(t, err, os.ErrNotExist, "chart must not be cached under the index digest")

	saved, _, err := c.DownloadTo(ref, "0.1.0", t.TempDir())
	require.NoError(t, err)
	got, err = os.ReadFile(saved)
	require.NoError(t, err)
	assert.Equal(t, layer, sha256.Sum256(got))
}
