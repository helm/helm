package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kubernetes/helm/pkg/common"
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

func TestCreateDeployments(t *testing.T) {
	c := stubContext()
	tpl := &common.Template{Name: "foo"}
	s := httpHarness(c, "POST /deployments", createDeploymentHandlerFunc)
	defer s.Close()

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(tpl); err != nil {
		t.Fatal(err)
	}

	res, err := http.Post(s.URL+"/deployments", "application/json", &b)
	if err != nil {
		t.Errorf("Failed POST: %s", err)
	} else if res.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, res.StatusCode)
	}
}
