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

package repotest

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"           // used for docker test registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"golang.org/x/crypto/bcrypt"
	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	ociRegistry "helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
)

func BasicAuthMiddleware(t *testing.T) http.HandlerFunc {
	t.Helper()
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "username" || password != "password" {
			t.Errorf("Expected request to use basic auth and for username == 'username' and password == 'password', got '%v', '%s', '%s'", ok, username, password)
		}
	})
}

type ServerOption func(*testing.T, *Server)

func WithTLSConfig(tlsConfig *tls.Config) ServerOption {
	return func(_ *testing.T, server *Server) {
		server.tlsConfig = tlsConfig
	}
}

func WithMiddleware(middleware http.HandlerFunc) ServerOption {
	return func(_ *testing.T, server *Server) {
		server.middleware = middleware
	}
}

func WithChartSourceGlob(glob string) ServerOption {
	return func(_ *testing.T, server *Server) {
		server.chartSourceGlob = glob
	}
}

// Server is an implementation of a repository server for testing.
type Server struct {
	docroot         string
	srv             *httptest.Server
	middleware      http.HandlerFunc
	tlsConfig       *tls.Config
	chartSourceGlob string
}

// NewTempServer creates a server inside of a temp dir.
//
// If the passed in string is not "", it will be treated as a shell glob, and files
// will be copied from that path to the server's docroot.
//
// The server is started automatically. The caller is responsible for stopping
// the server.
//
// The temp dir will be removed by testing package automatically when test finished.
func NewTempServer(t *testing.T, options ...ServerOption) *Server {
	t.Helper()
	docrootTempDir := t.TempDir()

	srv := newServer(t, docrootTempDir, options...)

	t.Cleanup(func() { os.RemoveAll(srv.docroot) })

	if srv.chartSourceGlob != "" {
		if _, err := srv.CopyCharts(srv.chartSourceGlob); err != nil {
			t.Fatal(err)
		}
	}

	return srv
}

// Create the server, but don't yet start it
func newServer(t *testing.T, docroot string, options ...ServerOption) *Server {
	t.Helper()
	absdocroot, err := filepath.Abs(docroot)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{
		docroot: absdocroot,
	}

	for _, option := range options {
		option(t, s)
	}

	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.middleware != nil {
			s.middleware.ServeHTTP(w, r)
		}
		http.FileServer(http.Dir(s.Root())).ServeHTTP(w, r)
	}))

	s.start()

	// Add the testing repository as the only repo. Server must be started for the server's URL to be valid
	if err := setTestingRepository(s.URL(), filepath.Join(s.docroot, "repositories.yaml")); err != nil {
		t.Fatal(err)
	}

	return s
}

type OCIServer struct {
	*registry.Registry
	RegistryURL  string
	Dir          string
	TestUsername string
	TestPassword string
	Client       *ociRegistry.Client
}

type OCIServerRunConfig struct {
	DependingChart *chart.Chart
}

type OCIServerOpt func(config *OCIServerRunConfig)

type OCIServerRunResult struct {
	PushedChart *ociRegistry.PushResult
}

func WithDependingChart(c *chart.Chart) OCIServerOpt {
	return func(config *OCIServerRunConfig) {
		config.DependingChart = c
	}
}

func NewOCIServer(t *testing.T, dir string) (*OCIServer, error) {
	t.Helper()
	testHtpasswdFileBasename := "authtest.htpasswd"
	testUsername, testPassword := "username", "password"

	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal("error generating bcrypt password for test htpasswd file")
	}
	htpasswdPath := filepath.Join(dir, testHtpasswdFileBasename)
	err = os.WriteFile(htpasswdPath, fmt.Appendf(nil, "%s:%s\n", testUsername, string(pwBytes)), 0o644)
	if err != nil {
		t.Fatalf("error creating test htpasswd file")
	}

	// Registry config
	config := &configuration.Configuration{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("error finding free port for test registry")
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	config.HTTP.Addr = ln.Addr().String()
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": "localhost",
			"path":  htpasswdPath,
		},
	}

	registryURL := fmt.Sprintf("localhost:%d", port)

	r, err := registry.NewRegistry(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}

	return &OCIServer{
		Registry:     r,
		RegistryURL:  registryURL,
		TestUsername: testUsername,
		TestPassword: testPassword,
		Dir:          dir,
	}, nil
}

func (srv *OCIServer) Run(t *testing.T, opts ...OCIServerOpt) {
	t.Helper()
	_ = srv.RunWithReturn(t, opts...)
}

func (srv *OCIServer) RunWithReturn(t *testing.T, opts ...OCIServerOpt) *OCIServerRunResult {
	t.Helper()
	cfg := &OCIServerRunConfig{}
	for _, fn := range opts {
		fn(cfg)
	}

	go srv.ListenAndServe()

	credentialsFile := filepath.Join(srv.Dir, "config.json")

	// init test client
	registryClient, err := ociRegistry.NewClient(
		ociRegistry.ClientOptDebug(true),
		ociRegistry.ClientOptEnableCache(true),
		ociRegistry.ClientOptWriter(os.Stdout),
		ociRegistry.ClientOptCredentialsFile(credentialsFile),
	)
	if err != nil {
		t.Fatalf("error creating registry client")
	}

	err = registryClient.Login(
		srv.RegistryURL,
		ociRegistry.LoginOptBasicAuth(srv.TestUsername, srv.TestPassword),
		ociRegistry.LoginOptInsecure(true),
		ociRegistry.LoginOptPlainText(true))
	if err != nil {
		t.Fatalf("error logging into registry with good credentials: %v", err)
	}

	ref := fmt.Sprintf("%s/u/ocitestuser/oci-dependent-chart:0.1.0", srv.RegistryURL)

	err = chartutil.ExpandFile(srv.Dir, filepath.Join(srv.Dir, "oci-dependent-chart-0.1.0.tgz"))
	if err != nil {
		t.Fatal(err)
	}

	// valid chart
	ch, err := loader.LoadDir(filepath.Join(srv.Dir, "oci-dependent-chart"))
	if err != nil {
		t.Fatal("error loading chart")
	}

	err = os.RemoveAll(filepath.Join(srv.Dir, "oci-dependent-chart"))
	if err != nil {
		t.Fatal("error removing chart before push")
	}

	// save it back to disk..
	absPath, err := chartutil.Save(ch, srv.Dir)
	if err != nil {
		t.Fatal("could not create chart archive")
	}

	// load it into memory...
	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatal("could not load chart into memory")
	}

	result, err := registryClient.Push(contentBytes, ref)
	if err != nil {
		t.Fatalf("error pushing dependent chart: %s", err)
	}
	t.Logf("Manifest.Digest: %s, Manifest.Size: %d, "+
		"Config.Digest: %s, Config.Size: %d, "+
		"Chart.Digest: %s, Chart.Size: %d",
		result.Manifest.Digest, result.Manifest.Size,
		result.Config.Digest, result.Config.Size,
		result.Chart.Digest, result.Chart.Size)

	srv.Client = registryClient
	c := cfg.DependingChart
	if c == nil {
		return &OCIServerRunResult{
			PushedChart: result,
		}
	}

	dependingRef := fmt.Sprintf("%s/u/ocitestuser/%s:%s",
		srv.RegistryURL, c.Metadata.Name, c.Metadata.Version)

	// load it into memory...
	absPath = filepath.Join(srv.Dir,
		fmt.Sprintf("%s-%s.tgz", c.Metadata.Name, c.Metadata.Version))
	contentBytes, err = os.ReadFile(absPath)
	if err != nil {
		t.Fatal("could not load chart into memory")
	}

	result, err = registryClient.Push(contentBytes, dependingRef)
	if err != nil {
		t.Fatalf("error pushing depending chart: %s", err)
	}
	t.Logf("Manifest.Digest: %s, Manifest.Size: %d, "+
		"Config.Digest: %s, Config.Size: %d, "+
		"Chart.Digest: %s, Chart.Size: %d",
		result.Manifest.Digest, result.Manifest.Size,
		result.Config.Digest, result.Config.Size,
		result.Chart.Digest, result.Chart.Size)

	return &OCIServerRunResult{
		PushedChart: result,
	}
}

// Root gets the docroot for the server.
func (s *Server) Root() string {
	return s.docroot
}

// CopyCharts takes a glob expression and copies those charts to the server root.
func (s *Server) CopyCharts(origin string) ([]string, error) {
	files, err := filepath.Glob(origin)
	if err != nil {
		return []string{}, err
	}
	copied := make([]string, len(files))
	for i, f := range files {
		base := filepath.Base(f)
		newname := filepath.Join(s.docroot, base)
		data, err := os.ReadFile(f)
		if err != nil {
			return []string{}, err
		}
		if err := os.WriteFile(newname, data, 0o644); err != nil {
			return []string{}, err
		}
		copied[i] = newname
	}

	err = s.CreateIndex()
	return copied, err
}

// CreateIndex will read docroot and generate an index.yaml file.
func (s *Server) CreateIndex() error {
	// generate the index
	index, err := repo.IndexDirectory(s.docroot, s.URL())
	if err != nil {
		return err
	}

	d, err := yaml.Marshal(index)
	if err != nil {
		return err
	}

	ifile := filepath.Join(s.docroot, "index.yaml")
	return os.WriteFile(ifile, d, 0o644)
}

func (s *Server) start() {
	if s.tlsConfig != nil {
		s.srv.TLS = s.tlsConfig
		s.srv.StartTLS()
	} else {
		s.srv.Start()
	}
}

// Stop stops the server and closes all connections.
//
// It should be called explicitly.
func (s *Server) Stop() {
	s.srv.Close()
}

// URL returns the URL of the server.
//
// Example:
//
//	http://localhost:1776
func (s *Server) URL() string {
	return s.srv.URL
}

func (s *Server) Client() *http.Client {
	return s.srv.Client()
}

// LinkIndices links the index created with CreateIndex and makes a symbolic link to the cache index.
//
// This makes it possible to simulate a local cache of a repository.
func (s *Server) LinkIndices() error {
	lstart := filepath.Join(s.docroot, "index.yaml")
	ldest := filepath.Join(s.docroot, "test-index.yaml")
	return os.Symlink(lstart, ldest)
}

// setTestingRepository sets up a testing repository.yaml with only the given URL.
func setTestingRepository(url, fname string) error {
	if url == "" {
		panic("no url")
	}

	r := repo.NewFile()
	r.Add(&repo.Entry{
		Name: "test",
		URL:  url,
	})
	return r.WriteFile(fname, 0o640)
}
