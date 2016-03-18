package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/kubernetes/helm/cmd/manager/router"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/httputil"
	"github.com/kubernetes/helm/pkg/registry"
)

func TestHealthz(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /", healthz)
	defer s.Close()

	res, err := http.Get(s.URL)
	if err != nil {
		t.Fatalf("Failed to GET healthz: %s", err)
	} else if res.StatusCode != 200 {
		t.Fatalf("Unexpected status: %d", res.StatusCode)
	}

	// TODO: Get the body and check on the content type and the body.
}

// httpHarness is a simple test server fixture.
// Simple fixture for standing up a test server with a single route.
//
// You must Close() the returned server.
func httpHarness(c *router.Context, route string, fn router.HandlerFunc) *httptest.Server {
	h := router.NewHandler(c)
	h.Add(route, fn)
	return httptest.NewServer(h)
}

// stubContext creates a stub of a Context object.
//
// This creates a stub context with the following properties:
// - Config is initialized to empty values
// - Encoder is initialized to httputil.DefaultEncoder
// - CredentialProvider is initialized to registry.InmemCredentialProvider
// - Manager is initialized to mockManager.
func stubContext() *router.Context {
	return &router.Context{
		Config:             &router.Config{},
		Manager:            &mockManager{},
		CredentialProvider: registry.NewInmemCredentialProvider(),
		Encoder:            httputil.DefaultEncoder,
	}
}

type mockManager struct{}

func (m *mockManager) ListDeployments() ([]common.Deployment, error) {
	return []common.Deployment{}, nil
}
func (m *mockManager) GetDeployment(name string) (*common.Deployment, error) {
	return &common.Deployment{}, nil
}
func (m *mockManager) CreateDeployment(t *common.Template) (*common.Deployment, error) {
	return &common.Deployment{}, nil
}
func (m *mockManager) DeleteDeployment(name string, forget bool) (*common.Deployment, error) {
	return &common.Deployment{}, nil
}
func (m *mockManager) PutDeployment(name string, t *common.Template) (*common.Deployment, error) {
	return &common.Deployment{}, nil
}

func (m *mockManager) ListManifests(deploymentName string) (map[string]*common.Manifest, error) {
	return map[string]*common.Manifest{}, nil
}
func (m *mockManager) GetManifest(deploymentName string, manifest string) (*common.Manifest, error) {
	return &common.Manifest{}, nil
}
func (m *mockManager) Expand(t *common.Template) (*common.Manifest, error) {
	return &common.Manifest{}, nil
}

func (m *mockManager) ListTypes() ([]string, error) {
	return []string{}, nil
}
func (m *mockManager) ListInstances(typeName string) ([]*common.TypeInstance, error) {
	return []*common.TypeInstance{}, nil
}
func (m *mockManager) GetRegistryForType(typeName string) (string, error) {
	return "", nil
}
func (m *mockManager) GetMetadataForType(typeName string) (string, error) {
	return "", nil
}

func (m *mockManager) ListRegistries() ([]*common.Registry, error) {
	return []*common.Registry{}, nil
}
func (m *mockManager) CreateRegistry(pr *common.Registry) error {
	return nil
}
func (m *mockManager) GetRegistry(name string) (*common.Registry, error) {
	return &common.Registry{}, nil
}
func (m *mockManager) DeleteRegistry(name string) error {
	return nil
}

func (m *mockManager) ListRegistryTypes(registryName string, regex *regexp.Regexp) ([]registry.Type, error) {
	return []registry.Type{}, nil
}
func (m *mockManager) GetDownloadURLs(registryName string, t registry.Type) ([]*url.URL, error) {
	return []*url.URL{}, nil
}
func (m *mockManager) GetFile(registryName string, url string) (string, error) {
	return "", nil
}
func (m *mockManager) CreateCredential(name string, c *common.RegistryCredential) error {
	return nil
}
func (m *mockManager) GetCredential(name string) (*common.RegistryCredential, error) {
	return &common.RegistryCredential{}, nil
}
