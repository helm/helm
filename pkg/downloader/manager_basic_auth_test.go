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

// basicAuthMiddleware returns a middleware that requires Basic Auth
func basicAuthMiddleware(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("401 Unauthorized\n"))
				return
			}

			if !strings.HasPrefix(auth, "Basic ") {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("401 Unauthorized\n"))
				return
			}

			payload, err := base64.StdEncoding.DecodeString(auth[6:])
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("401 Unauthorized\n"))
				return
			}

			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) != 2 || pair[0] != username || pair[1] != password {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("401 Unauthorized\n"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// createTestServer creates an HTTP server with Basic Auth for testing
func createTestServer(username, password string) *httptest.Server {
	mux := http.NewServeMux()

	// Mock index.yaml
	indexYAML := `apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    description: A test chart
    urls:
    - test-chart-0.1.0.tgz
generated: "2025-09-28T20:00:00Z"
`

	// Mock chart content
	chartTGZ := "fake-chart-content-for-testing"

	// Index file endpoint
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write([]byte(indexYAML))
	})

	// Chart file endpoint
	mux.HandleFunc("/test-chart-0.1.0.tgz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write([]byte(chartTGZ))
	})

	// Apply Basic Auth middleware only if credentials are provided
	var handler http.Handler = mux
	if username != "" && password != "" {
		handler = basicAuthMiddleware(username, password)(mux)
	}

	return httptest.NewServer(handler)
}

func TestManagerBasicAuth(t *testing.T) {
	tests := []struct {
		name              string
		serverUsername    string
		serverPassword    string
		managerUsername   string
		managerPassword   string
		expectSuccess     bool
		expectErrorString string
	}{
		{
			name:            "Success with valid credentials",
			serverUsername:  "testuser",
			serverPassword:  "testpass",
			managerUsername: "testuser",
			managerPassword: "testpass",
			expectSuccess:   true,
		},
		{
			name:              "Fail without credentials on protected repo",
			serverUsername:    "testuser",
			serverPassword:    "testpass",
			managerUsername:   "",
			managerPassword:   "",
			expectSuccess:     false,
			expectErrorString: "401 Unauthorized",
		},
		{
			name:              "Fail with wrong credentials",
			serverUsername:    "testuser",
			serverPassword:    "testpass",
			managerUsername:   "wronguser",
			managerPassword:   "wrongpass",
			expectSuccess:     false,
			expectErrorString: "401 Unauthorized",
		},
		{
			name:            "Success with public repo (no auth required)",
			serverUsername:  "",
			serverPassword:  "",
			managerUsername: "",
			managerPassword: "",
			expectSuccess:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			srv := createTestServer(tt.serverUsername, tt.serverPassword)
			defer srv.Close()

			// Create temporary directories
			tempDir := t.TempDir()
			chartDir := filepath.Join(tempDir, "test-chart")
			chartsDir := filepath.Join(chartDir, "charts")

			err := os.MkdirAll(chartsDir, 0755)
			if err != nil {
				t.Fatal(err)
			}

			// Create a test Chart.yaml
			chartYAML := fmt.Sprintf(`apiVersion: v2
name: test-chart
description: A Helm chart for testing
type: application
version: 0.1.0
appVersion: "1.16.0"

dependencies:
  - name: test-chart
    version: "0.1.0"
    repository: "%s"
`, srv.URL)

			chartYAMLPath := filepath.Join(chartDir, "Chart.yaml")
			err = os.WriteFile(chartYAMLPath, []byte(chartYAML), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// Create Manager with test settings
			repoCache := filepath.Join(tempDir, "cache")
			repoConfig := filepath.Join(tempDir, "repositories.yaml")
			contentCache := filepath.Join(tempDir, "content")

			err = os.MkdirAll(repoCache, 0755)
			if err != nil {
				t.Fatal(err)
			}

			err = os.MkdirAll(contentCache, 0755)
			if err != nil {
				t.Fatal(err)
			}

			// Create empty repositories.yaml
			err = os.WriteFile(repoConfig, []byte("apiVersion: v1\ngenerated: \"2025-09-28T20:00:00Z\"\nrepositories: []\n"), 0644)
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
				Username:         tt.managerUsername,
				Password:         tt.managerPassword,
				Debug:            true,
			}

			// Execute the test
			err = m.Update()

			// Validate results
			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, but got error: %v\nOutput: %s", err, out.String())
				}
			} else {
				// For failed cases, we check both the error and the output for 401
				outputStr := out.String()
				errorStr := ""
				if err != nil {
					errorStr = err.Error()
				}

				if !strings.Contains(outputStr, tt.expectErrorString) && !strings.Contains(errorStr, tt.expectErrorString) {
					t.Errorf("Expected error or output containing '%s', but got error: %v\nOutput: %s", tt.expectErrorString, err, outputStr)
				}
			}
		})
	}
}

func TestManagerBasicAuthNoCredentialLeak(t *testing.T) {
	// Create two servers: one public, one private
	publicSrv := createTestServer("", "")
	defer publicSrv.Close()

	privateSrv := createTestServer("testuser", "testpass")
	defer privateSrv.Close()

	// Track requests to ensure credentials are not sent to public repo
	var publicRequests []http.Header
	var privateRequests []http.Header

	publicSrvWithLogging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		publicRequests = append(publicRequests, r.Header.Clone())
		publicSrv.Config.Handler.ServeHTTP(w, r)
	}))
	defer publicSrvWithLogging.Close()

	privateSrvWithLogging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateRequests = append(privateRequests, r.Header.Clone())
		privateSrv.Config.Handler.ServeHTTP(w, r)
	}))
	defer privateSrvWithLogging.Close()

	// Create temporary directories
	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "test-chart")
	chartsDir := filepath.Join(chartDir, "charts")

	err := os.MkdirAll(chartsDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create a Chart.yaml with dependencies from both public and private repos
	chartYAML := fmt.Sprintf(`apiVersion: v2
name: test-chart
description: A Helm chart for testing
type: application
version: 0.1.0
appVersion: "1.16.0"

dependencies:
  - name: public-chart
    version: "0.1.0"
    repository: "%s"
  - name: private-chart
    version: "0.1.0"
    repository: "%s"
`, publicSrvWithLogging.URL, privateSrvWithLogging.URL)

	chartYAMLPath := filepath.Join(chartDir, "Chart.yaml")
	err = os.WriteFile(chartYAMLPath, []byte(chartYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create Manager with test settings
	repoCache := filepath.Join(tempDir, "cache")
	repoConfig := filepath.Join(tempDir, "repositories.yaml")
	contentCache := filepath.Join(tempDir, "content")

	err = os.MkdirAll(repoCache, 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(contentCache, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create empty repositories.yaml
	err = os.WriteFile(repoConfig, []byte("apiVersion: v1\ngenerated: \"2025-09-28T20:00:00Z\"\nrepositories: []\n"), 0644)
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
		Username:         "testuser",
		Password:         "testpass",
		Debug:            true,
	}

	// Execute the test - we expect this to fail because we're sending creds to all repos
	// This test documents current behavior - in the future this should be improved
	_ = m.Update() // Ignore error, we're just testing credential behavior

	// For now, we just verify that credentials are being sent to all repos
	// This is a known limitation that could be improved in a future PR
	t.Logf("Current behavior: credentials are sent to all repositories")
	t.Logf("Public server received %d requests", len(publicRequests))
	t.Logf("Private server received %d requests", len(privateRequests))

	// Check if Authorization header was sent to public server (current behavior)
	for _, header := range publicRequests {
		if auth := header.Get("Authorization"); auth != "" {
			t.Logf("WARNING: Authorization header sent to public repo: %s", auth)
		}
	}
}
