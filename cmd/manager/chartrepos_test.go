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
	"github.com/kubernetes/helm/pkg/repo"

	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
)

var (
	//	TestRepoURL            = "gs://kubernetes-charts-testing"
	TestRepoURL            = "foo"
	TestChartName          = "frobnitz-0.0.1.tgz"
	TestRepoType           = string(repo.GCSRepoType)
	TestRepoFormat         = string(repo.GCSRepoFormat)
	TestRepoCredentialName = "default"
)

func TestListChartRepos(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /repositories", listChartReposHandlerFunc)
	defer s.Close()

	URL := getTestURL(t, s.URL, "", "")
	res, err := http.Get(URL)
	if err != nil {
		t.Fatalf("Failed GET: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}

func TestGetChartRepo(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /repositories/*", getChartRepoHandlerFunc)
	defer s.Close()

	URL := getTestURL(t, s.URL, url.QueryEscape(TestRepoURL), "")
	res, err := http.Get(URL)
	if err != nil {
		t.Fatalf("Failed GET: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}

func TestListRepoCharts(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /repositories/*/charts", listRepoChartsHandlerFunc)
	defer s.Close()

	URL := getTestURL(t, s.URL, url.QueryEscape(TestRepoURL), "charts")
	res, err := http.Get(URL)
	if err != nil {
		t.Fatalf("Failed GET: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}

func TestGetRepoChart(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /repositories/*/charts/*", getRepoChartHandlerFunc)
	defer s.Close()

	chartURL := fmt.Sprintf("charts/%s", TestChartName)
	URL := getTestURL(t, s.URL, url.QueryEscape(TestRepoURL), chartURL)
	res, err := http.Get(URL)
	if err != nil {
		t.Fatalf("Failed GET: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}

func TestAddChartRepo(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "POST /repositories", addChartRepoHandlerFunc)
	defer s.Close()

	URL := getTestURL(t, s.URL, "", "")
	body := getTestRepo(t, URL)
	res, err := http.Post(URL, "application/json", body)
	if err != nil {
		t.Fatalf("Failed POST: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}

func TestRemoveChartRepo(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "DELETE /repositories/*", removeChartRepoHandlerFunc)
	defer s.Close()

	URL := getTestURL(t, s.URL, url.QueryEscape(TestRepoURL), "")
	req, err := http.NewRequest("DELETE", URL, nil)
	if err != nil {
		t.Fatalf("Cannot create DELETE request: %s", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed DELETE: %s", err)
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}

func getTestRepo(t *testing.T, URL string) io.Reader {
	tr, err := repo.NewRepo(URL, TestRepoCredentialName, TestRepoFormat, TestRepoType)
	if err != nil {
		t.Fatalf("Cannot create test repository: %s", err)
	}

	trb, err := json.Marshal(&tr)
	if err != nil {
		t.Fatalf("Cannot marshal test repository: %s", err)
	}

	return bytes.NewReader(trb)
}

func getTestURL(t *testing.T, baseURL, repoURL, chartURL string) string {
	URL := fmt.Sprintf("%s/repositories", baseURL)
	if repoURL != "" {
		URL = fmt.Sprintf("%s/%s", URL, repoURL)
	}

	if chartURL != "" {
		URL = fmt.Sprintf("%s/%s", URL, chartURL)
	}

	u, err := url.Parse(URL)
	if err != nil {
		t.Fatalf("cannot parse test URL %s: %s", URL, err)
	}

	return u.String()
}
