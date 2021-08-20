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
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/repo/repotest"
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
		{name: "reference, testing-relative-trailing-slash repo", ref: "testing-relative-trailing-slash/foo", expect: "http://example.com/helm/charts/foo-1.2.3.tgz"},
		{name: "reference, testing-relative-trailing-slash repo", ref: "testing-relative-trailing-slash/bar", expect: "http://example.com/helm/bar-1.2.3.tgz"},
		{name: "full URL, HTTPS, irrelevant version", ref: "https://example.com/foo-1.2.3.tgz", version: "0.1.0", expect: "https://example.com/foo-1.2.3.tgz", fail: true},
		{name: "full URL, file", ref: "file:///foo-1.2.3.tgz", fail: true},
		{name: "invalid", ref: "invalid-1.2.3", fail: true},
		{name: "not found", ref: "nosuchthing/invalid-1.2.3", fail: true},
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
		u, err := c.ResolveChartVersion(tt.ref, tt.version)
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

		u, err := c.ResolveChartVersion(tt.ref, tt.version)
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
	v, err := VerifyChart("testdata/signtest-0.1.0.tgz", "testdata/helm-test-key.pub")
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
	srv := repotest.NewTempServerWithCleanupAndBasicAuth(t, "testdata/*.tgz*")
	defer srv.Stop()
	if err := srv.CreateIndex(); err != nil {
		t.Fatal(err)
	}

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyAlways,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
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
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/*.tgz*")
	srv.Stop()
	if err != nil {
		t.Fatal(err)
	}
	srv.StartTLS()
	defer srv.Stop()
	if err := srv.CreateIndex(); err != nil {
		t.Fatal(err)
	}
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	repoConfig := filepath.Join(srv.Root(), "repositories.yaml")
	repoCache := srv.Root()

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyAlways,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}),
		Options: []getter.Option{},
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
	defer ensure.HelmHome(t)()

	dest := ensure.TempDir(t)
	defer os.RemoveAll(dest)

	// Set up a fake repo
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

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
	if _, err = c.scanReposForURL(u, rf); err != ErrNoOwnerRepo {
		t.Fatalf("expected ErrNoOwnerRepo, got %v", err)
	}
}
