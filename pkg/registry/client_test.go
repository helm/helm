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

package registry

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
)

// Inspired by oras test
// https://github.com/oras-project/oras-go/blob/05a2b09cbf2eab1df691411884dc4df741ec56ab/content_test.go#L1802
func TestTagManifestTransformsReferences(t *testing.T) {
	memStore := memory.New()
	client := &Client{out: io.Discard}
	ctx := t.Context()

	refWithPlus := "test-registry.io/charts/test:1.0.0+metadata"
	expectedRef := "test-registry.io/charts/test:1.0.0_metadata" // + becomes _

	configDesc := ocispec.Descriptor{MediaType: ConfigMediaType, Digest: "sha256:config", Size: 100}
	layers := []ocispec.Descriptor{{MediaType: ChartLayerMediaType, Digest: "sha256:layer", Size: 200}}

	parsedRef, err := newReference(refWithPlus)
	require.NoError(t, err)

	desc, err := client.tagManifest(ctx, memStore, configDesc, layers, nil, parsedRef)
	require.NoError(t, err)

	transformedDesc, err := memStore.Resolve(ctx, expectedRef)
	require.NoError(t, err, "Should find the reference with _ instead of +")
	require.Equal(t, desc.Digest, transformedDesc.Digest)

	_, err = memStore.Resolve(ctx, refWithPlus)
	require.Error(t, err, "Should NOT find the reference with the original +")
}

// Verifies that Login always restores ForceAttemptOAuth2 to false on success.
func TestLogin_ResetsForceAttemptOAuth2_OnSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			// Accept either HEAD or GET
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	if c.authorizer == nil || c.authorizer.ForceAttemptOAuth2 {
		t.Fatalf("expected ForceAttemptOAuth2 default to be false")
	}

	// Call Login with plain HTTP against our test server
	if err := c.Login(host, LoginOptPlainText(true), LoginOptBasicAuth("u", "p")); err != nil {
		t.Fatalf("Login error: %v", err)
	}

	if c.authorizer.ForceAttemptOAuth2 {
		t.Errorf("ForceAttemptOAuth2 should be false after successful Login")
	}
}

// Verifies that Login restores ForceAttemptOAuth2 to false even when ping fails.
func TestLogin_ResetsForceAttemptOAuth2_OnFailure(t *testing.T) {
	t.Parallel()

	// Start and immediately close, so connections will fail
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	host := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	// Invoke Login, expect an error but ForceAttemptOAuth2 must end false
	_ = c.Login(host, LoginOptPlainText(true), LoginOptBasicAuth("u", "p"))

	if c.authorizer.ForceAttemptOAuth2 {
		t.Errorf("ForceAttemptOAuth2 should be false after failed Login")
	}
}

// TestWarnIfHostHasPath verifies that warnIfHostHasPath correctly detects path components.
func TestWarnIfHostHasPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		wantWarn bool
	}{
		{
			name:     "domain only",
			host:     "ghcr.io",
			wantWarn: false,
		},
		{
			name:     "domain with port",
			host:     "localhost:8000",
			wantWarn: false,
		},
		{
			name:     "domain with repository path",
			host:     "ghcr.io/terryhowe",
			wantWarn: true,
		},
		{
			name:     "domain with nested path",
			host:     "ghcr.io/terryhowe/myrepo",
			wantWarn: true,
		},
		{
			name:     "localhost with port and path",
			host:     "localhost:8000/myrepo",
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := warnIfHostHasPath(tt.host)
			if got != tt.wantWarn {
				t.Errorf("warnIfHostHasPath(%q) = %v, want %v", tt.host, got, tt.wantWarn)
			}
		})
	}
}

// TestPushConcurrent verifies that concurrent Push operations on the same Client
// do not interfere with each other. This test is designed to catch race conditions
// when run with -race flag.
func TestPushConcurrent(t *testing.T) {
	t.Parallel()

	// Create a mock registry server that accepts pushes
	var mu sync.Mutex
	uploads := make(map[string][]byte)
	var uploadCounter int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/blobs/"):
			// Blob existence check - return 404 to force upload
			w.WriteHeader(http.StatusNotFound)

		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/blobs/uploads/"):
			// Start upload - return upload URL with unique ID
			mu.Lock()
			uploadCounter++
			uploadID := fmt.Sprintf("upload-%d", uploadCounter)
			mu.Unlock()
			w.Header().Set("Location", fmt.Sprintf("%s%s", r.URL.Path, uploadID))
			w.WriteHeader(http.StatusAccepted)

		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/blobs/uploads/"):
			// Complete upload - extract digest from query param
			body, _ := io.ReadAll(r.Body)
			digest := r.URL.Query().Get("digest")
			mu.Lock()
			uploads[r.URL.Path] = body
			mu.Unlock()
			w.Header().Set("Docker-Content-Digest", digest)
			w.WriteHeader(http.StatusCreated)

		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/manifests/"):
			// Manifest push - compute actual sha256 digest of the body
			body, _ := io.ReadAll(r.Body)
			hash := sha256.Sum256(body)
			digest := fmt.Sprintf("sha256:%x", hash)
			w.Header().Set("Docker-Content-Digest", digest)
			w.WriteHeader(http.StatusCreated)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	// Create client
	credFile := filepath.Join(t.TempDir(), "config.json")
	client, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
		ClientOptPlainHTTP(),
	)
	require.NoError(t, err)

	// Load test chart
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	require.NoError(t, err, "no error loading test chart")

	meta, err := extractChartMeta(chartData)
	require.NoError(t, err, "no error extracting chart meta")

	// Run concurrent pushes
	const numGoroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Each goroutine pushes to a different tag to avoid conflicts
			ref := fmt.Sprintf("%s/testrepo/%s:%s-%d", host, meta.Name, meta.Version, idx)
			_, err := client.Push(chartData, ref, PushOptStrictMode(false))
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	// Check for errors
	for err := range errs {
		t.Error(err)
	}
}
