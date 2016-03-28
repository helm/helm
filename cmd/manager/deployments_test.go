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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/kubernetes/helm/pkg/common"
)

func TestHealthz(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /", healthz)
	defer s.Close()

	res, err := http.Get(s.URL)
	if err != nil {
		t.Fatalf("err on http get: %v", err)
	}
	body, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		t.Fatalf("Failed to GET healthz: %s", err)
	} else if res.StatusCode != 200 {
		t.Fatalf("Unexpected status: %d", res.StatusCode)
	}

	expectedBody := "OK"
	if bytes.Equal(body, []byte(expectedBody)) {
		t.Fatalf("Expected response body: %s, Actual response body: %s",
			expectedBody, string(body))
	}

	expectedContentType := "text/plain"
	contentType := res.Header["Content-Type"][0]
	if !strings.Contains(contentType, expectedContentType) {
		t.Fatalf("Expected Content-Type to include %s", expectedContentType)
	}
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

func TestListDeployments(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /deployments", listDeploymentsHandlerFunc)
	defer s.Close()

	man := c.Manager.(*mockManager)
	man.deployments = []*common.Deployment{
		{Name: "one", State: &common.DeploymentState{Status: common.CreatedStatus}},
		{Name: "two", State: &common.DeploymentState{Status: common.DeployedStatus}},
	}

	res, err := http.Get(s.URL + "/deployments")
	if err != nil {
		t.Errorf("Failed GET: %s", err)
	} else if res.StatusCode != http.StatusOK {
		t.Errorf("Unexpected status code: %d", res.StatusCode)
	}

	var out []string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Errorf("Failed to parse results: %s", err)
		return
	}
	if len(out) != 2 {
		t.Errorf("Expected 2 names, got %d", len(out))
	}
}

func TestGetDeployments(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /deployments/*", getDeploymentHandlerFunc)
	defer s.Close()

	man := c.Manager.(*mockManager)
	man.deployments = []*common.Deployment{
		{Name: "portunes", State: &common.DeploymentState{Status: common.CreatedStatus}},
	}

	res, err := http.Get(s.URL + "/deployments/portunes")
	if err != nil {
		t.Errorf("Failed GET: %s", err)
	} else if res.StatusCode != http.StatusOK {
		t.Errorf("Unexpected status code: %d", res.StatusCode)
	}

	var out common.Deployment
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Errorf("Failed to parse results: %s", err)
		return
	}

	if out.Name != "portunes" {
		t.Errorf("Unexpected name %q", out.Name)
	}

	if out.State.Status != common.CreatedStatus {
		t.Errorf("Unexpected status %d", out.State.Status)
	}
}
