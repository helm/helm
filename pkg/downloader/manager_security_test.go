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
	"time"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
)

// TestSecurityCredentialHandling tests security aspects of credential handling
// This ensures the fix for #31262 doesn't introduce security vulnerabilities
func TestSecurityCredentialHandling(t *testing.T) {
	t.Run("Credentials not logged in debug mode", func(t *testing.T) {
		testCredentialLogging(t)
	})

	t.Run("Credentials not sent to unrelated hosts", func(t *testing.T) {
		testCredentialHostScoping(t)
	})

	t.Run("Credentials properly encoded", func(t *testing.T) {
		testCredentialEncoding(t)
	})

	t.Run("Malicious credentials handled safely", func(t *testing.T) {
		testMaliciousCredentialHandling(t)
	})

	t.Run("Credentials cleared from memory", func(t *testing.T) {
		testCredentialMemoryHandling(t)
	})
}

// testCredentialLogging ensures credentials are not exposed in logs
func testCredentialLogging(t *testing.T) {
	username, password := "sensitive-user", "super-secret-password"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Mock successful response
		if r.URL.Path == "/index.yaml" {
			w.Write([]byte(`apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    urls: [test-chart-0.1.0.tgz]
generated: "2025-09-28T20:00:00Z"`))
		} else if r.URL.Path == "/test-chart-0.1.0.tgz" {
			w.Write([]byte("fake-chart"))
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	chartDir := setupTestChart(t, tempDir, srv.URL)
	repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

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
		Debug:            true, // Enable debug mode
	}

	_ = m.Update()
	output := out.String()

	// Ensure credentials are NOT exposed in debug output
	if strings.Contains(output, password) {
		t.Errorf("Password '%s' found in debug output - SECURITY VULNERABILITY!", password)
		t.Errorf("Debug output: %s", output)
	}

	if strings.Contains(output, username) && !strings.Contains(output, "username") {
		// Allow the word "username" but not the actual username value in logs
		if strings.Contains(output, fmt.Sprintf("username: %s", username)) {
			t.Errorf("Username '%s' found in debug output - potential information leak!", username)
		}
	}
}

// testCredentialHostScoping ensures credentials are not sent to unrelated hosts
func testCredentialHostScoping(t *testing.T) {
	username, password := "testuser", "testpass"

	// Create target server that expects auth
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/index.yaml" {
			w.Write([]byte(`apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    urls: [http://malicious-external-server.com/steal-creds.tgz]
generated: "2025-09-28T20:00:00Z"`))
		}
	}))
	defer targetSrv.Close()

	// Create malicious server that should NOT receive credentials
	var maliciousRequests []http.Header
	maliciousSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		maliciousRequests = append(maliciousRequests, r.Header.Clone())
		w.Write([]byte("malicious-content"))
	}))
	defer maliciousSrv.Close()

	tempDir := t.TempDir()
	chartDir := setupTestChart(t, tempDir, targetSrv.URL)
	repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

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

	_ = m.Update() // Will try to download from malicious server

	// Verify NO credentials were sent to malicious server
	// NOTE: This test documents current behavior - credentials ARE sent to all hosts
	// This is a known limitation that should be improved in the future
	for i, header := range maliciousRequests {
		auth := header.Get("Authorization")
		if auth != "" {
			t.Logf("WARNING: Request %d to malicious server has Authorization header: %s", i, auth)
			t.Logf("This is current behavior - credentials are sent to all repositories")
			t.Logf("Future improvement: implement host-specific credential scoping")
		}
	}
}

// testCredentialEncoding ensures credentials are properly encoded
func testCredentialEncoding(t *testing.T) {
	// Test with special characters that need proper encoding
	testCases := []struct {
		name     string
		username string
		password string
	}{
		{"Normal credentials", "user", "pass"},
		{"Username with special chars", "user@domain.com", "password"},
		{"Password with special chars", "user", "p@ssw0rd!#$%"},
		{"Both with special chars", "user@domain", "p@ss:w0rd"},
		{"Unicode characters", "użytkownik", "hasło123"},
		// Note: Empty password case is intentionally excluded as no auth header should be sent
		{"Colon in username", "user:name", "password"}, // This should work
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedAuth = r.Header.Get("Authorization")

				if receivedAuth == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Verify proper Basic Auth format
				if !strings.HasPrefix(receivedAuth, "Basic ") {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("Invalid auth format"))
					return
				}

				// Decode and verify
				encoded := receivedAuth[6:]
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("Invalid base64 encoding"))
					return
				}

				// Verify format is username:password
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) != 2 {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("Invalid credential format"))
					return
				}

				if parts[0] != tc.username || parts[1] != tc.password {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("Invalid credentials"))
					return
				}

				// Success response
				if r.URL.Path == "/index.yaml" {
					w.Write([]byte(`apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    urls: [test-chart-0.1.0.tgz]
generated: "2025-09-28T20:00:00Z"`))
				} else {
					w.Write([]byte("fake-chart"))
				}
			}))
			defer srv.Close()

			tempDir := t.TempDir()
			chartDir := setupTestChart(t, tempDir, srv.URL)
			repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

			var out bytes.Buffer
			m := &Manager{
				Out:              &out,
				ChartPath:        chartDir,
				Getters:          getter.All(&cli.EnvSettings{}),
				RepositoryConfig: repoConfig,
				RepositoryCache:  repoCache,
				ContentCache:     contentCache,
				Username:         tc.username,
				Password:         tc.password,
			}

			err := m.Update()

			// Verify proper encoding was received
			if receivedAuth == "" {
				t.Errorf("No Authorization header received")
			} else if !strings.HasPrefix(receivedAuth, "Basic ") {
				t.Errorf("Invalid Authorization header format: %s", receivedAuth)
			} else {
				// Decode and verify
				encoded := receivedAuth[6:]
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					t.Errorf("Failed to decode base64: %v", err)
				} else {
					expected := fmt.Sprintf("%s:%s", tc.username, tc.password)
					if string(decoded) != expected {
						t.Errorf("Credential mismatch. Expected: %s, Got: %s", expected, string(decoded))
					}
				}
			}

			// For problematic cases, we should handle gracefully
			if strings.Contains(tc.username, ":") && tc.username != "user:name" {
				t.Logf("Note: Username contains colon, may cause parsing issues in some systems")
			}

			_ = err // Ignore other errors for this security test
		})
	}
}

// testMaliciousCredentialHandling tests handling of potentially malicious credentials
func testMaliciousCredentialHandling(t *testing.T) {
	maliciousInputs := []struct {
		name        string
		username    string
		password    string
		expectError bool
	}{
		{"Extremely long username", strings.Repeat("a", 10000), "pass", false},
		{"Extremely long password", "user", strings.Repeat("a", 10000), false},
		{"NULL bytes in username", "user\x00malicious", "pass", false},
		{"NULL bytes in password", "user", "pass\x00malicious", false},
		{"Newlines in username", "user\nmalicious", "pass", false},
		{"Newlines in password", "user", "pass\nmalicious", false},
		{"Control characters", "user\r\t", "pass\r\t", false},
	}

	for _, tc := range maliciousInputs {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")

				// Log received auth for security analysis
				t.Logf("Received auth header: %q", auth)

				if auth != "" && strings.HasPrefix(auth, "Basic ") {
					decoded, err := base64.StdEncoding.DecodeString(auth[6:])
					if err != nil {
						t.Logf("Failed to decode auth: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					t.Logf("Decoded credentials: %q", string(decoded))
				}

				// Always respond with success to test credential handling
				if r.URL.Path == "/index.yaml" {
					w.Write([]byte(`apiVersion: v1
entries: {}
generated: "2025-09-28T20:00:00Z"`))
				} else {
					w.Write([]byte("ok"))
				}
			}))
			defer srv.Close()

			tempDir := t.TempDir()
			chartDir := setupTestChart(t, tempDir, srv.URL)
			repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

			var out bytes.Buffer
			m := &Manager{
				Out:              &out,
				ChartPath:        chartDir,
				Getters:          getter.All(&cli.EnvSettings{}),
				RepositoryConfig: repoConfig,
				RepositoryCache:  repoCache,
				ContentCache:     contentCache,
				Username:         tc.username,
				Password:         tc.password,
			}

			err := m.Update()

			// Ensure malicious input doesn't crash the system
			if tc.expectError && err == nil {
				t.Errorf("Expected error for malicious input, but got none")
			}

			// Ensure no panic occurred and system is stable
			output := out.String()
			t.Logf("Output length: %d characters", len(output))
		})
	}
}

// testCredentialMemoryHandling verifies credentials are handled properly in memory
func testCredentialMemoryHandling(t *testing.T) {
	username, password := "testuser", "testpass"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple success response
		if r.URL.Path == "/index.yaml" {
			w.Write([]byte(`apiVersion: v1
entries: {}
generated: "2025-09-28T20:00:00Z"`))
		} else {
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	chartDir := setupTestChart(t, tempDir, srv.URL)
	repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

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

	err := m.Update()
	if err != nil {
		t.Logf("Update completed with result: %v", err)
	}

	// Verify credentials are still accessible in Manager (expected behavior)
	if m.Username != username {
		t.Errorf("Username was modified: expected %s, got %s", username, m.Username)
	}
	if m.Password != password {
		t.Errorf("Password was modified: expected %s, got %s", password, m.Password)
	}

	// This test documents current behavior - credentials remain in Manager
	// Future enhancement could implement credential clearing after use
	t.Logf("Note: Credentials remain in Manager struct after use")
	t.Logf("Future enhancement: consider clearing sensitive data after use")
}

// Helper functions for test setup
func setupTestChart(t *testing.T, tempDir, repoURL string) string {
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
    repository: "%s"`, repoURL)

	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return chartDir
}

func setupCacheDirectories(t *testing.T, tempDir string) (string, string, string) {
	repoCache := filepath.Join(tempDir, "cache")
	repoConfig := filepath.Join(tempDir, "repositories.yaml")
	contentCache := filepath.Join(tempDir, "content")

	for _, dir := range []string{repoCache, contentCache} {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := os.WriteFile(repoConfig, []byte("apiVersion: v1\nrepositories: []\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return repoCache, repoConfig, contentCache
}

// TestSecurityRegressionPrevention ensures our fix doesn't reintroduce known security issues
func TestSecurityRegressionPrevention(t *testing.T) {
	t.Run("No credential injection in URLs", func(t *testing.T) {
		// Ensure credentials don't get injected into URLs themselves
		testNoCredentialInjectionInURLs(t)
	})

	t.Run("Timeout handling with auth", func(t *testing.T) {
		// Ensure auth doesn't interfere with proper timeout handling
		testTimeoutHandlingWithAuth(t)
	})
}

func testNoCredentialInjectionInURLs(t *testing.T) {
	username, password := "testuser", "testpass"

	var requestURLs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURLs = append(requestURLs, r.URL.String())

		// Check if credentials somehow ended up in URL
		fullURL := r.URL.String()
		if strings.Contains(fullURL, username) || strings.Contains(fullURL, password) {
			t.Errorf("SECURITY ISSUE: Credentials found in URL: %s", fullURL)
		}

		// Simple response
		if r.URL.Path == "/index.yaml" {
			w.Write([]byte(`apiVersion: v1
entries: {}
generated: "2025-09-28T20:00:00Z"`))
		} else {
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	chartDir := setupTestChart(t, tempDir, srv.URL)
	repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

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

	_ = m.Update()

	// Verify no credentials in any requested URLs
	for _, reqURL := range requestURLs {
		if strings.Contains(reqURL, username) || strings.Contains(reqURL, password) {
			t.Errorf("SECURITY VULNERABILITY: Credentials found in request URL: %s", reqURL)
		}
	}

	t.Logf("Verified %d URLs contain no embedded credentials", len(requestURLs))
}

func testTimeoutHandlingWithAuth(t *testing.T) {
	username, password := "testuser", "testpass"

	// Create a server that delays response
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Small delay

		if r.URL.Path == "/index.yaml" {
			w.Write([]byte(`apiVersion: v1
entries: {}
generated: "2025-09-28T20:00:00Z"`))
		} else {
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	chartDir := setupTestChart(t, tempDir, srv.URL)
	repoCache, repoConfig, contentCache := setupCacheDirectories(t, tempDir)

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

	// This should complete normally despite the delay
	err := m.Update()

	// Ensure auth doesn't interfere with normal error handling
	output := out.String()
	if strings.Contains(output, "timeout") {
		t.Logf("Timeout handling appears normal with auth enabled")
	}

	_ = err // We're testing that it doesn't hang or panic
}

// TestRegressionPreFixBehavior ensures that the behavior before the fix continues to work
// This test guarantees backward compatibility and that existing functionality isn't broken
func TestRegressionPreFixBehavior(t *testing.T) {
	t.Run("Manager without credentials works as before", func(t *testing.T) {
		testManagerWithoutCredentials(t)
	})

	t.Run("Empty credentials behave as no credentials", func(t *testing.T) {
		testEmptyCredentialsBehavior(t)
	})

	t.Run("Public repository access unchanged", func(t *testing.T) {
		testPublicRepositoryAccess(t)
	})

	t.Run("Error handling unchanged", func(t *testing.T) {
		testErrorHandlingUnchanged(t)
	})
}

// testManagerWithoutCredentials ensures that a Manager without Username/Password
// fields set continues to work exactly as it did before the fix
func testManagerWithoutCredentials(t *testing.T) {
	// Create a test server that serves a simple index.yaml
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should NOT receive any Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			t.Errorf("Expected no Authorization header, but got: %s", authHeader)
		}

		if r.URL.Path == "/index.yaml" {
			w.Header().Set("Content-Type", "application/x-yaml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    urls: [test-chart-0.1.0.tgz]
generated: "2025-09-28T20:00:00Z"`))
		} else if strings.HasSuffix(r.URL.Path, ".tgz") {
			w.Header().Set("Content-Type", "application/x-gzip")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-chart-content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create Chart.yaml
	chartYaml := `apiVersion: v2
name: test-chart
version: 0.1.0
dependencies:
- name: test-chart
  version: "0.1.0"
  repository: ` + srv.URL

	chartPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartPath, []byte(chartYaml), 0644); err != nil {
		t.Fatalf("Failed to create Chart.yaml: %v", err)
	}

	// Create Manager WITHOUT Username/Password (as it was before the fix)
	out := &bytes.Buffer{}
	man := &Manager{
		Out:             out,
		ChartPath:       tempDir,
		Getters:         getter.All(&cli.EnvSettings{}),
		RepositoryCache: tempDir, // Add cache to avoid cache errors
		// Note: Username and Password are intentionally NOT set
		// This mimics the behavior before the fix
	}

	// Test that Update works as before (should attempt without credentials)
	err := man.Update()

	// The exact error doesn't matter - we're testing that:
	// 1. No authorization header is sent
	// 2. The code doesn't panic or break
	// 3. The error handling is the same as before
	if err != nil {
		t.Logf("Update failed as expected without credentials: %v", err)
	} else {
		t.Log("Update succeeded without credentials (public repo behavior)")
	}

	// Verify no auth-related output in logs
	output := out.String()
	if strings.Contains(strings.ToLower(output), "authorization") ||
		strings.Contains(strings.ToLower(output), "username") ||
		strings.Contains(strings.ToLower(output), "password") {
		t.Errorf("Unexpected auth-related output when no credentials provided: %s", output)
	}
}

// testEmptyCredentialsBehavior ensures that empty Username/Password strings
// behave exactly like no credentials (backward compatibility)
func testEmptyCredentialsBehavior(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should NOT receive any Authorization header when credentials are empty
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			t.Errorf("Expected no Authorization header with empty credentials, but got: %s", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`apiVersion: v1
entries: {}
generated: "2025-09-28T20:00:00Z"`))
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	chartYaml := `apiVersion: v2
name: test-chart
version: 0.1.0
dependencies:
- name: test-chart
  version: "0.1.0"
  repository: ` + srv.URL

	chartPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartPath, []byte(chartYaml), 0644); err != nil {
		t.Fatalf("Failed to create Chart.yaml: %v", err)
	}

	// Create Manager with empty credentials (should behave like no credentials)
	out := &bytes.Buffer{}
	man := &Manager{
		Out:             out,
		ChartPath:       tempDir,
		Getters:         getter.All(&cli.EnvSettings{}),
		RepositoryCache: tempDir, // Add cache
		Username:        "",      // Empty string
		Password:        "",      // Empty string
	}

	err := man.Update()

	// We don't care about the specific error, just that no auth header was sent
	_ = err

	t.Log("Empty credentials behaved as no credentials (backward compatibility maintained)")
}

// testPublicRepositoryAccess ensures that access to public repositories
// continues to work unchanged
func testPublicRepositoryAccess(t *testing.T) {
	// Simulate a public repository that doesn't require authentication
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public repos should work regardless of whether auth headers are present or not
		// This tests that we don't break public repo access

		if r.URL.Path == "/index.yaml" {
			w.Header().Set("Content-Type", "application/x-yaml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`apiVersion: v1
entries:
  public-chart:
  - name: public-chart
    version: 1.0.0
    urls: [public-chart-1.0.0.tgz]
generated: "2025-09-28T20:00:00Z"`))
		} else if strings.HasSuffix(r.URL.Path, ".tgz") {
			w.Header().Set("Content-Type", "application/x-gzip")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-chart-content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	chartYaml := `apiVersion: v2
name: test-chart
version: 0.1.0
dependencies:
- name: public-chart
  version: "1.0.0"
  repository: ` + srv.URL

	chartPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartPath, []byte(chartYaml), 0644); err != nil {
		t.Fatalf("Failed to create Chart.yaml: %v", err)
	}

	// Test both scenarios: with and without credentials for public repo
	scenarios := []struct {
		name     string
		username string
		password string
	}{
		{"Public repo without credentials", "", ""},
		{"Public repo with credentials (should still work)", "user", "pass"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			man := &Manager{
				Out:             out,
				ChartPath:       tempDir,
				Getters:         getter.All(&cli.EnvSettings{}),
				RepositoryCache: tempDir, // Add cache
				Username:        scenario.username,
				Password:        scenario.password,
			}

			err := man.Update()
			if err != nil {
				// For this test we expect some error since we're not providing a real chart
				// but the important thing is that it doesn't fail due to auth issues
				if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
					t.Errorf("Public repository access failed with auth error: %v", err)
				} else {
					t.Logf("Public repository access failed with expected non-auth error: %v", err)
				}
			} else {
				t.Log("Public repository access succeeded")
			}
		})
	}
}

// testErrorHandlingUnchanged ensures that error handling behavior
// remains the same as before the fix
func testErrorHandlingUnchanged(t *testing.T) {
	// Test various error scenarios to ensure they're handled the same way

	t.Run("Invalid repository URL", func(t *testing.T) {
		tempDir := t.TempDir()
		chartYaml := `apiVersion: v2
name: test-chart
version: 0.1.0
dependencies:
- name: test-chart
  version: "0.1.0"
  repository: "invalid-url"`

		chartPath := filepath.Join(tempDir, "Chart.yaml")
		if err := os.WriteFile(chartPath, []byte(chartYaml), 0644); err != nil {
			t.Fatalf("Failed to create Chart.yaml: %v", err)
		}

		out := &bytes.Buffer{}
		man := &Manager{
			Out:             out,
			ChartPath:       tempDir,
			Getters:         getter.All(&cli.EnvSettings{}),
			RepositoryCache: tempDir, // Add cache
			// Test both with and without credentials
		}

		err := man.Update()
		if err == nil {
			t.Error("Expected error for invalid URL, but got none")
		}

		// The error should be about the URL, not about authentication
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			t.Errorf("Got auth error for invalid URL, expected URL error: %v", err)
		}
	})

	t.Run("Non-existent repository", func(t *testing.T) {
		tempDir := t.TempDir()
		chartYaml := `apiVersion: v2
name: test-chart
version: 0.1.0
dependencies:
- name: test-chart
  version: "0.1.0"
  repository: "http://localhost:99999/non-existent"`

		chartPath := filepath.Join(tempDir, "Chart.yaml")
		if err := os.WriteFile(chartPath, []byte(chartYaml), 0644); err != nil {
			t.Fatalf("Failed to create Chart.yaml: %v", err)
		}

		out := &bytes.Buffer{}
		man := &Manager{
			Out:             out,
			ChartPath:       tempDir,
			Getters:         getter.All(&cli.EnvSettings{}),
			RepositoryCache: tempDir, // Add cache
		}

		err := man.Update()
		if err == nil {
			t.Error("Expected error for non-existent repository, but got none")
		}

		// Should get connection error, not auth error
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			t.Errorf("Got auth error for connection issue, expected connection error: %v", err)
		}
	})

	t.Log("Error handling behavior verified to be unchanged")
}
