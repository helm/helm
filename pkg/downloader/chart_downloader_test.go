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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	helmregistry "helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/chart/loader"
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
			t.Errorf("%s: failed with error %s", tt.name, err)
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
	// Set up a fake repo with basic auth enabled
	srv, err := repotest.NewTempServer("testdata/*.tgz*")
	srv.Stop()
	if err != nil {
		t.Fatal(err)
	}
	srv.WithMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "username" || password != "password" {
			t.Errorf("Expected request to use basic auth and for username == 'username' and password == 'password', got '%v', '%s', '%s'", ok, username, password)
		}
	}))
	srv.Start()
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
	srv, err := repotest.NewTempServer("testdata/*.tgz*")
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

	// Set up a fake repo
	srv, err := repotest.NewTempServer("testdata/*.tgz*")
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

func TestDownloadToFromOCIRepository(t *testing.T) {
	var (
		CredentialsFileBasename  = "config.json"
		testCacheRootDir         = "helm-registry-test"
		testHtpasswdFileBasename = "authtest.htpasswd"
		testUsername             = "myuser"
		testPassword             = "mypass"
	)
	os.RemoveAll(testCacheRootDir)
	os.Mkdir(testCacheRootDir, 0700)

	var out bytes.Buffer
	credentialsFile := filepath.Join(testCacheRootDir, CredentialsFileBasename)

	client, err := auth.NewClient(credentialsFile)
	assert.Nil(t, err, "no error creating auth client")

	resolver, err := client.Resolver(context.Background(), http.DefaultClient, false)
	assert.Nil(t, err, "no error creating resolver")

	// create cache
	cache, err := helmregistry.NewCache(
		helmregistry.CacheOptDebug(true),
		helmregistry.CacheOptWriter(&out),
		helmregistry.CacheOptRoot(filepath.Join(testCacheRootDir, helmregistry.CacheRootDir)),
	)

	assert.Nil(t, err, "failed creating cache")

	// init test client
	registryClient, err := helmregistry.NewClient(
		helmregistry.ClientOptDebug(true),
		helmregistry.ClientOptWriter(&out),
		helmregistry.ClientOptAuthorizer(&helmregistry.Authorizer{
			Client: client,
		}),
		helmregistry.ClientOptResolver(&helmregistry.Resolver{
			Resolver: resolver,
		}),
		helmregistry.ClientOptCache(cache),
	)
	assert.Nil(t, err, "failed creating registry client")

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	assert.Nil(t, err, "no error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(testCacheRootDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	assert.Nil(t, err, "error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	assert.Nil(t, err, "no error finding free port for test registry")
	dockerRegistryHost := fmt.Sprintf("localhost:%d", port)
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": "localhost",
			"path":  htpasswdPath,
		},
	}
	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	assert.Nil(t, err, "no error creating test registry")

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
	err = registryClient.Login(dockerRegistryHost, testUsername, testPassword, false)
	assert.Nil(t, err, "failed to login to registry with username "+testUsername+" and password "+testPassword)

	ref, _ := helmregistry.ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", dockerRegistryHost))
	ch, err := loader.LoadDir("testdata/local-subchart")
	assert.Nil(t, err, "failed to load local chart")
	err = registryClient.SaveChart(ch, ref)
	assert.Nil(t, err, "failed to save chart")
	err = registryClient.PushChart(ref)
	assert.Nil(t, err, "failed to push chart")

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyIfPossible,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: append(getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}), helmregistry.NewRegistryGetterProvider(registryClient)),
		Options: []getter.Option{
			getter.WithBasicAuth("username", "password"),
		},
	}
	// the filename becomes the {last segment of the image name}-{the image tag}
	fname := "/testchart-1.2.3.tgz"
	dest := ensure.TempDir(t)
	where, _, err := c.DownloadTo(fmt.Sprintf("oci://%s/testrepo/testchart:1.2.3", dockerRegistryHost), "", dest)
	if err != nil {
		t.Fatal(err)
	}

	if expect := filepath.Join(dest, fname); where != expect {
		t.Errorf("Expected download to %s, got %s", expect, where)
	}

	if _, err := os.Stat(filepath.Join(dest, fname)); err != nil {
		t.Error(err)
	}

	os.RemoveAll(testCacheRootDir)
}

func TestDownloadToFromOCIRepositoryWithoutTag(t *testing.T) {
	var (
		CredentialsFileBasename  = "config.json"
		testCacheRootDir         = "helm-registry-test"
		testHtpasswdFileBasename = "authtest.htpasswd"
		testUsername             = "myuser"
		testPassword             = "mypass"
	)
	os.RemoveAll(testCacheRootDir)
	os.Mkdir(testCacheRootDir, 0700)

	var out bytes.Buffer
	credentialsFile := filepath.Join(testCacheRootDir, CredentialsFileBasename)

	client, err := auth.NewClient(credentialsFile)
	assert.Nil(t, err, "no error creating auth client")

	resolver, err := client.Resolver(context.Background(), http.DefaultClient, false)
	assert.Nil(t, err, "no error creating resolver")

	// create cache
	cache, err := helmregistry.NewCache(
		helmregistry.CacheOptDebug(true),
		helmregistry.CacheOptWriter(&out),
		helmregistry.CacheOptRoot(filepath.Join(testCacheRootDir, helmregistry.CacheRootDir)),
	)

	assert.Nil(t, err, "failed creating cache")

	// init test client
	registryClient, err := helmregistry.NewClient(
		helmregistry.ClientOptDebug(true),
		helmregistry.ClientOptWriter(&out),
		helmregistry.ClientOptAuthorizer(&helmregistry.Authorizer{
			Client: client,
		}),
		helmregistry.ClientOptResolver(&helmregistry.Resolver{
			Resolver: resolver,
		}),
		helmregistry.ClientOptCache(cache),
	)
	assert.Nil(t, err, "failed creating registry client")

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	assert.Nil(t, err, "no error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(testCacheRootDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	assert.Nil(t, err, "error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	assert.Nil(t, err, "no error finding free port for test registry")
	dockerRegistryHost := fmt.Sprintf("localhost:%d", port)
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": "localhost",
			"path":  htpasswdPath,
		},
	}
	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	assert.Nil(t, err, "no error creating test registry")

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
	err = registryClient.Login(dockerRegistryHost, testUsername, testPassword, false)
	assert.Nil(t, err, "failed to login to registry with username "+testUsername+" and password "+testPassword)

	ref, _ := helmregistry.ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", dockerRegistryHost))
	ch, err := loader.LoadDir("testdata/local-subchart")
	assert.Nil(t, err, "failed to load local chart")
	err = registryClient.SaveChart(ch, ref)
	assert.Nil(t, err, "failed to save chart")
	err = registryClient.PushChart(ref)
	assert.Nil(t, err, "failed to push chart")

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyIfPossible,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: append(getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}), helmregistry.NewRegistryGetterProvider(registryClient)),
		Options: []getter.Option{
			getter.WithBasicAuth("username", "password"),
		},
	}
	// the filename becomes the {last segment of the image name}-{the image tag}
	fname := "/testchart-1.2.3.tgz"
	dest := ensure.TempDir(t)
	version := "1.2.3"
	where, _, err := c.DownloadTo(fmt.Sprintf("oci://%s/testrepo/testchart", dockerRegistryHost), version, dest)
	if err != nil {
		t.Fatal(err)
	}

	if expect := filepath.Join(dest, fname); where != expect {
		t.Errorf("Expected download to %s, got %s", expect, where)
	}

	if _, err := os.Stat(filepath.Join(dest, fname)); err != nil {
		t.Error(err)
	}

	os.RemoveAll(testCacheRootDir)
}

func TestDownloadToFromOCIRepositoryWithoutTagOrVersion(t *testing.T) {
	var (
		CredentialsFileBasename = "config.json"
		testCacheRootDir        = "helm-registry-test"
	)
	os.RemoveAll(testCacheRootDir)
	os.Mkdir(testCacheRootDir, 0700)

	var out bytes.Buffer
	credentialsFile := filepath.Join(testCacheRootDir, CredentialsFileBasename)

	client, err := auth.NewClient(credentialsFile)
	assert.Nil(t, err, "no error creating auth client")

	resolver, err := client.Resolver(context.Background(), http.DefaultClient, false)
	assert.Nil(t, err, "no error creating resolver")

	// create cache
	cache, err := helmregistry.NewCache(
		helmregistry.CacheOptDebug(true),
		helmregistry.CacheOptWriter(&out),
		helmregistry.CacheOptRoot(filepath.Join(testCacheRootDir, helmregistry.CacheRootDir)),
	)

	assert.Nil(t, err, "failed creating cache")

	// init test client
	registryClient, err := helmregistry.NewClient(
		helmregistry.ClientOptDebug(true),
		helmregistry.ClientOptWriter(&out),
		helmregistry.ClientOptAuthorizer(&helmregistry.Authorizer{
			Client: client,
		}),
		helmregistry.ClientOptResolver(&helmregistry.Resolver{
			Resolver: resolver,
		}),
		helmregistry.ClientOptCache(cache),
	)

	assert.Nil(t, err, "failed creating registry client")

	c := ChartDownloader{
		Out:              os.Stderr,
		Verify:           VerifyIfPossible,
		Keyring:          "testdata/helm-test-key.pub",
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		Getters: append(getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
		}), helmregistry.NewRegistryGetterProvider(registryClient)),
		Options: []getter.Option{
			getter.WithBasicAuth("username", "password"),
		},
	}
	// the filename becomes the {last segment of the image name}-{the image tag}
	dest := ensure.TempDir(t)
	_, _, err = c.DownloadTo("oci://testrepo/testchart", "", dest)
	if err == nil {
		t.Error("download succeeded without version or tag")
	}

	os.RemoveAll(testCacheRootDir)
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
