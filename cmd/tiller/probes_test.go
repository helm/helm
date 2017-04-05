/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

func TestPrometheus(t *testing.T) {
	mux := http.NewServeMux()
	addPrometheusHandler(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics returned an error (%s)", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics returned status code %d, expected %d", resp.StatusCode, http.StatusOK)
	}
}
