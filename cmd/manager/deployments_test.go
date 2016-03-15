package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubernetes/deployment-manager/cmd/manager/router"
)

func TestHealthz(t *testing.T) {
	c := mockContext()
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

func mockContext() *router.Context {
	// TODO: We need mocks for credentials and manager.
	return &router.Context{}
}
