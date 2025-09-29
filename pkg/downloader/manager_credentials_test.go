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
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
)

// TestIssue31262 specifically tests the bug reported in https://github.com/helm/helm/issues/31262
// This test reproduces the exact scenario described in the issue:
// helm dependency update with --username/--password should work with non-OCI repositories
func TestIssue31262(t *testing.T) {
	// Create a test server that requires Basic Auth (like the private Artifactory in the issue)
	username, password := "testuser", "testpass"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request for debugging
		t.Logf("Request: %s %s, Auth header: %s", r.Method, r.URL.Path, r.Header.Get("Authorization"))

		// Check for Basic Auth
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Private Repository"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized - Authentication required"))
			return
		}

		if !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized - Invalid auth method"))
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[6:])
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized - Invalid auth encoding"))
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != username || pair[1] != password {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized - Invalid credentials"))
			return
		}

		// Serve the requested resource
		switch r.URL.Path {
		case "/index.yaml":
			// Mock index.yaml similar to Artifactory
			indexYAML := `apiVersion: v1
entries:
  external-secrets:
  - name: external-secrets
    version: 0.19.2
    description: External Secrets Operator is a Kubernetes operator
    urls:
    - external-secrets-0.19.2.tgz
generated: "2025-09-28T20:00:00Z"
`
			w.Header().Set("Content-Type", "application/x-yaml")
			w.Write([]byte(indexYAML))
		case "/external-secrets-0.19.2.tgz":
			// Mock chart content
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte("mock-chart-content"))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer srv.Close()

	// Create temporary directories for the test
	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "test-chart")
	chartsDir := filepath.Join(chartDir, "charts")

	err := os.MkdirAll(chartsDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create Chart.yaml exactly like in the issue report
	chartYAML := fmt.Sprintf(`apiVersion: v2
name: test-chart
description: A Helm chart for Kubernetes
type: application
version: 0.1.0
appVersion: "1.16.0"

dependencies:
  - name: external-secrets
    version: "0.19.2"
    repository: "%s"
`, srv.URL)

	chartYAMLPath := filepath.Join(chartDir, "Chart.yaml")
	err = os.WriteFile(chartYAMLPath, []byte(chartYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create cache directories
	repoCache := filepath.Join(tempDir, "cache")
	repoConfig := filepath.Join(tempDir, "repositories.yaml")
	contentCache := filepath.Join(tempDir, "content")

	for _, dir := range []string{repoCache, contentCache} {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create empty repositories.yaml (simulating no pre-configured repos)
	err = os.WriteFile(repoConfig, []byte("apiVersion: v1\ngenerated: \"2025-09-28T20:00:00Z\"\nrepositories: []\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test Case 1: WITHOUT credentials (should fail with 401 like in the original issue)
	t.Run("Without credentials - should fail with 401", func(t *testing.T) {
		var out bytes.Buffer
		m := &Manager{
			Out:              &out,
			ChartPath:        chartDir,
			Getters:          getter.All(&cli.EnvSettings{}),
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
			Username:         "", // NO USERNAME
			Password:         "", // NO PASSWORD
		}

		_ = m.Update() // Error expected for this test case
		outputStr := out.String()

		// Should fail and contain 401 error
		if !strings.Contains(outputStr, "401 Unauthorized") {
			t.Errorf("Expected 401 Unauthorized error in output, but got: %s", outputStr)
		}

		t.Logf("Without credentials output (expected to fail): %s", outputStr)
	})

	// Test Case 2: WITH credentials (should succeed - this is the fix!)
	t.Run("With credentials - should succeed", func(t *testing.T) {
		var out bytes.Buffer
		m := &Manager{
			Out:              &out,
			ChartPath:        chartDir,
			Getters:          getter.All(&cli.EnvSettings{}),
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     contentCache,
			Username:         username, // WITH USERNAME
			Password:         password, // WITH PASSWORD
		}

		err := m.Update()
		outputStr := out.String()

		// Should NOT contain 401 error
		if strings.Contains(outputStr, "401 Unauthorized") {
			t.Errorf("Expected NO 401 Unauthorized error with valid credentials, but got: %s", outputStr)
		}

		// Should contain success messages
		if !strings.Contains(outputStr, "Successfully got an update from") {
			t.Errorf("Expected success message for index fetch, but got: %s", outputStr)
		}

		if !strings.Contains(outputStr, "Downloading external-secrets") {
			t.Errorf("Expected chart download message, but got: %s", outputStr)
		}

		t.Logf("With credentials output (expected to succeed): %s", outputStr)

		// The error (if any) should NOT be authentication-related
		if err != nil && strings.Contains(err.Error(), "401") {
			t.Errorf("Should not have 401 authentication error with valid credentials, but got: %v", err)
		}
	})
}

// TestManagerCredentialsArePassedToBothPhases ensures credentials are passed to both:
// 1. Index.yaml fetch (ensureMissingRepos -> parallelRepoUpdate)
// 2. Chart download (downloadAll -> findChartURL)
func TestManagerCredentialsArePassedToBothPhases(t *testing.T) {
	username, password := "testuser", "testpass"
	var indexRequests []http.Header
	var chartRequests []http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		// Verify authentication
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized"))
			return
		}

		payload, _ := base64.StdEncoding.DecodeString(auth[6:])
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != username || pair[1] != password {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized"))
			return
		}

		// Track requests by endpoint
		switch r.URL.Path {
		case "/index.yaml":
			indexRequests = append(indexRequests, r.Header.Clone())
			w.Header().Set("Content-Type", "application/x-yaml")
			w.Write([]byte(`apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    urls:
    - test-chart-0.1.0.tgz
generated: "2025-09-28T20:00:00Z"`))
		case "/test-chart-0.1.0.tgz":
			chartRequests = append(chartRequests, r.Header.Clone())
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte("mock-chart"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Setup test environment
	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "test-chart")
	err := os.MkdirAll(filepath.Join(chartDir, "charts"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	chartYAML := fmt.Sprintf(`apiVersion: v2
name: test-chart
description: Test chart
version: 0.1.0
dependencies:
  - name: test-chart
    version: "0.1.0"
    repository: "%s"`, srv.URL)

	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Setup cache directories
	repoCache := filepath.Join(tempDir, "cache")
	repoConfig := filepath.Join(tempDir, "repositories.yaml")
	contentCache := filepath.Join(tempDir, "content")

	for _, dir := range []string{repoCache, contentCache} {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = os.WriteFile(repoConfig, []byte("apiVersion: v1\nrepositories: []\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	m := &Manager{
		Out:              &out,
		ChartPath:        chartDir,
		Getters:          getter.All(&cli.EnvSettings{}),
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		ContentCache:     contentCache,
		Username:         username,
		Password:         password,
	}

	err = m.Update()

	// Verify both phases received authenticated requests
	if len(indexRequests) == 0 {
		t.Error("Expected at least one request to /index.yaml, but got none")
	}

	if len(chartRequests) == 0 {
		t.Error("Expected at least one request to chart download endpoint, but got none")
	}

	// Verify both requests had proper authentication
	for i, req := range indexRequests {
		auth := req.Get("Authorization")
		if auth == "" {
			t.Errorf("Index request %d missing Authorization header", i)
		} else if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Index request %d has invalid Authorization header: %s", i, auth)
		}
	}

	for i, req := range chartRequests {
		auth := req.Get("Authorization")
		if auth == "" {
			t.Errorf("Chart request %d missing Authorization header", i)
		} else if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Chart request %d has invalid Authorization header: %s", i, auth)
		}
	}

	t.Logf("Successfully verified credentials were passed to both phases:")
	t.Logf("- Index requests: %d", len(indexRequests))
	t.Logf("- Chart requests: %d", len(chartRequests))
}
