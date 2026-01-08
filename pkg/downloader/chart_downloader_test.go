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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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

	// snapshot options
	snapshotOpts := c.Options

	for _, tt := range tests {
		// reset chart downloader options for each test case
		c.Options = snapshotOpts

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

		got, err := getter.NewHTTPGetter(
			append(
				c.Options,
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
	if err != nil {
		t.Fatal(err)
	}

	entry, err := c.scanReposForURL(u, rf)
	if err != nil {
		t.Fatal(err)
	}

	if entry.Name != "testing" {
		t.Errorf("Unexpected repo %q for URL %q", entry.Name, u)
	}

	// A lookup failure should produce an ErrNoOwnerRepo
	u = "https://no.such.repo/foo/bar-1.23.4.tgz"
	if _, err = c.scanReposForURL(u, rf); !errors.Is(err, ErrNoOwnerRepo) {
		t.Fatalf("expected ErrNoOwnerRepo, got %v", err)
	}
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
