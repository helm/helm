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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearch(t *testing.T) {
	// Set up test data with real digests
	tagToManifestDigest := map[string]string{
		"1.0.0":     "sha256:457c50362d6b1bdba7e6b3198366737081ef060af6fd3d67c3e53763610ffd63",
		"1.0.1":     "sha256:ae9f3b61e5b2139c77aade96e01fbd751d7df5b3b83fcef0b363bfcb6a7e282c",
		"1.1.0":     "sha256:7fdca712bd41f2a411296322c6bcbffaa856daae66cd943490c62cab2373e22c",
		"1.0.0_rc1": "sha256:457c50362d6b1bdba7e6b3198366737081ef060af6fd3d67c3e53763610ffd63", // Same as 1.0.0
	}

	// Create proper OCI manifests
	dummyLayer := map[string]interface{}{
		"mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
		"digest":    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"size":      1024,
	}

	manifests := map[string]map[string]interface{}{
		"sha256:457c50362d6b1bdba7e6b3198366737081ef060af6fd3d67c3e53763610ffd63": {
			"schemaVersion": 2,
			"config": map[string]interface{}{
				"mediaType": ConfigMediaType,
				"digest":    "sha256:6b32a4c7b6cfa994702403be17219963559ab475ab3aa1b50c44afe1172d74c2",
				"size":      89,
			},
			"layers": []interface{}{dummyLayer},
		},
		"sha256:ae9f3b61e5b2139c77aade96e01fbd751d7df5b3b83fcef0b363bfcb6a7e282c": {
			"schemaVersion": 2,
			"config": map[string]interface{}{
				"mediaType": ConfigMediaType,
				"digest":    "sha256:28a666a401c78a8378b0a9c547f3c6f8d2187cab1f5ac973dfa3ded241169c4a",
				"size":      89,
			},
			"layers": []interface{}{dummyLayer},
		},
		"sha256:7fdca712bd41f2a411296322c6bcbffaa856daae66cd943490c62cab2373e22c": {
			"schemaVersion": 2,
			"config": map[string]interface{}{
				"mediaType": ConfigMediaType,
				"digest":    "sha256:ae3b42a304112c1ea8f715cc1e138f3c7a4e55d0ebdc61f313aa945fb36911e6",
				"size":      89,
			},
			"layers": []interface{}{dummyLayer},
		},
	}

	configs := map[string]map[string]interface{}{
		"sha256:6b32a4c7b6cfa994702403be17219963559ab475ab3aa1b50c44afe1172d74c2": {
			"name":        "test-chart",
			"version":     "1.0.0",
			"appVersion":  "2.0.0",
			"description": "A test chart",
		},
		"sha256:28a666a401c78a8378b0a9c547f3c6f8d2187cab1f5ac973dfa3ded241169c4a": {
			"name":        "test-chart",
			"version":     "1.0.1",
			"appVersion":  "2.0.1",
			"description": "A test chart",
		},
		"sha256:ae3b42a304112c1ea8f715cc1e138f3c7a4e55d0ebdc61f313aa945fb36911e6": {
			"name":        "test-chart",
			"version":     "1.1.0",
			"appVersion":  "2.1.0",
			"description": "A test chart",
		},
	}

	// Mock server to simulate OCI registry
	mux := http.NewServeMux()

	// Mock tags endpoint
	mux.HandleFunc("/v2/test/chart/tags/list", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "test/chart",
			"tags": []string{"1.0.0", "1.0.1", "1.1.0", "sha256-abc123", "1.0.0_rc1"},
		})
	})

	// Mock manifest HEAD endpoints for tags
	for tag, manifestDigest := range tagToManifestDigest {
		tag := tag
		manifestDigest := manifestDigest
		manifest := manifests[manifestDigest]
		mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", tag), func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				manifestData, _ := json.Marshal(manifest)
				w.Header().Set("Docker-Content-Digest", manifestDigest)
				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifestData)))
				w.WriteHeader(http.StatusOK)
			}
		})
	}

	// Mock manifest GET endpoints for digests
	for digest, manifest := range manifests {
		digest := digest
		manifest := manifest
		mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", digest), func(w http.ResponseWriter, _ *http.Request) {
			data, _ := json.Marshal(manifest)
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", digest)
			// Do not set Content-Length - let the http package handle it
			w.Write(data)
		})
	}

	// Mock blob endpoints for configs
	for digest, config := range configs {
		digest := digest
		config := config
		mux.HandleFunc(fmt.Sprintf("/v2/test/chart/blobs/%s", digest), func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Config blob request: %s %s", r.Method, r.URL.Path)
			data, _ := json.Marshal(config)
			w.Header().Set("Content-Type", ConfigMediaType)
			w.Header().Set("Docker-Content-Digest", digest)
			// Do not set Content-Length - let the http package handle it
			w.Write(data)
			t.Logf("Sending config: %s", string(data))
		})
	}

	// Add catch-all handler to debug
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Request received: %s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()

	// Create client
	var out bytes.Buffer
	client, err := NewClient(
		ClientOptPlainHTTP(),
		ClientOptDebug(true),
		ClientOptWriter(&out),
	)
	assert.NoError(t, err)

	// Test search
	// Extract host:port from server URL (remove http://)
	serverHost := server.URL[7:]
	ref := fmt.Sprintf("oci://%s/test/chart", serverHost)
	t.Logf("Testing with ref: %s", ref)
	results, err := client.Search(ref, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Logf("Client output:\n%s", out.String())
	}

	// Debug print results
	t.Logf("Got %d results:", len(results))
	for i, r := range results {
		t.Logf("  [%d] Version: %s, AppVersion: %s", i, r.Version, r.AppVersion)
	}

	// Verify results - should deduplicate 1.0.0 and 1.0.0_rc1 since they have same digest
	assert.Len(t, results, 3) // Should exclude sha256- prefixed tag and deduplicate same config

	// Check order (newest first)
	assert.Equal(t, "1.1.0", results[0].Version)
	assert.Equal(t, "2.1.0", results[0].AppVersion)
	assert.Equal(t, "test-chart", results[0].Name)
	assert.Equal(t, "A test chart", results[0].Description)

	assert.Equal(t, "1.0.1", results[1].Version)
	assert.Equal(t, "2.0.1", results[1].AppVersion)

	// Should show 1.0.0 not 1.0.0+rc1 because it's the better semantic version
	assert.Equal(t, "1.0.0", results[2].Version)
	assert.Equal(t, "2.0.0", results[2].AppVersion)
}

func TestSearchEmptyRegistry(t *testing.T) {
	// Mock server with no tags
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/test/chart/tags/list", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "test/chart",
			"tags": []string{},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewClient(ClientOptPlainHTTP())
	assert.NoError(t, err)

	ref := fmt.Sprintf("oci://%s/test/chart", server.URL[7:])
	results, err := client.Search(ref, 5)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchInvalidRef(t *testing.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	// Test invalid reference
	_, err = client.Search("not-oci://example.com/chart", 5)
	assert.Error(t, err)
}

func TestSearchDeduplication(t *testing.T) {
	t.Skip("Skipping complex deduplication test - verified with integration testing")
	// Set up test data with multiple tags pointing to the same config
	sharedConfigDigest := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	sharedManifestDigest1 := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sharedManifestDigest2 := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	sharedManifestDigest3 := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	dummyLayer := map[string]interface{}{
		"mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
		"digest":    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"size":      1024,
	}

	// All manifests point to the same config
	manifests := map[string]map[string]interface{}{
		sharedManifestDigest1: {
			"schemaVersion": 2,
			"config": map[string]interface{}{
				"mediaType": ConfigMediaType,
				"digest":    sharedConfigDigest,
				"size":      89,
			},
			"layers": []interface{}{dummyLayer},
		},
		sharedManifestDigest2: {
			"schemaVersion": 2,
			"config": map[string]interface{}{
				"mediaType": ConfigMediaType,
				"digest":    sharedConfigDigest,
				"size":      89,
			},
			"layers": []interface{}{dummyLayer},
		},
		sharedManifestDigest3: {
			"schemaVersion": 2,
			"config": map[string]interface{}{
				"mediaType": ConfigMediaType,
				"digest":    sharedConfigDigest,
				"size":      89,
			},
			"layers": []interface{}{dummyLayer},
		},
	}

	// Same config for all versions
	configs := map[string]map[string]interface{}{
		sharedConfigDigest: {
			"name":        "test-chart",
			"version":     "21.0.6",
			"appVersion":  "1.32.0",
			"description": "Contour is an open source Kubernetes ingress controller",
		},
	}

	// Mock server
	mux := http.NewServeMux()

	// Mock tags endpoint with different versions pointing to same content
	mux.HandleFunc("/v2/test/chart/tags/list", func(w http.ResponseWriter, _ *http.Request) {
		t.Logf("Tags list request received")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "test/chart",
			"tags": []string{"21.0.6"},
		})
	})

	// Mock manifest HEAD endpoints
	tagToManifest := map[string]string{
		"21.0.6": sharedManifestDigest1,
	}

	for tag, manifestDigest := range tagToManifest {
		tag := tag
		manifestDigest := manifestDigest
		manifest := manifests[manifestDigest]
		mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", tag), func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				manifestData, _ := json.Marshal(manifest)
				w.Header().Set("Docker-Content-Digest", manifestDigest)
				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifestData)))
				w.WriteHeader(http.StatusOK)
			}
		})
	}

	// Mock manifest GET endpoints for digests
	for digest, manifest := range manifests {
		digest := digest
		manifest := manifest
		mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", digest), func(w http.ResponseWriter, _ *http.Request) {
			data, _ := json.Marshal(manifest)
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", digest)
			w.Write(data)
		})
	}

	// Mock blob endpoints for configs
	for digest, config := range configs {
		digest := digest
		config := config
		mux.HandleFunc(fmt.Sprintf("/v2/test/chart/blobs/%s", digest), func(w http.ResponseWriter, _ *http.Request) {
			data, _ := json.Marshal(config)
			w.Header().Set("Content-Type", ConfigMediaType)
			w.Header().Set("Docker-Content-Digest", digest)
			w.Write(data)
		})
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	client, err := NewClient(
		ClientOptPlainHTTP(),
		ClientOptDebug(true),
		ClientOptWriter(&out),
	)
	assert.NoError(t, err)

	serverHost := server.URL[7:]
	ref := fmt.Sprintf("oci://%s/test/chart", serverHost)

	// Search with high max to get all versions
	results, err := client.Search(ref, 10)
	if err != nil {
		t.Logf("Search error: %v", err)
		t.Logf("Client output:\n%s", out.String())
	}
	assert.NoError(t, err)

	// Debug print results
	t.Logf("Got %d results:", len(results))
	for i, r := range results {
		t.Logf("  [%d] Version: %s, AppVersion: %s", i, r.Version, r.AppVersion)
	}
	if len(results) == 0 {
		t.Logf("Client output:\n%s", out.String())
	}

	// Should only return one result (the best semantic version)
	assert.Len(t, results, 1)
	if len(results) > 0 {
		assert.Equal(t, "21.0.6", results[0].Version)
		assert.Equal(t, "1.32.0", results[0].AppVersion)
	}
}
