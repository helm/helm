package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbesServer(t *testing.T) {
	mux := newProbesMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/readiness")
	if err != nil {
		t.Fatalf("GET /readiness returned an error (%s)", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /readiness returned status code %d, expected %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Get(srv.URL + "/liveness")
	if err != nil {
		t.Fatalf("GET /liveness returned an error (%s)", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /liveness returned status code %d, expected %d", resp.StatusCode, http.StatusOK)
	}
}
