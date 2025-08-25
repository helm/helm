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

package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/internal/test"
)

func TestSearchRepositoriesCmd(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	tests := []cmdTestCase{{
		name:   "search for 'alpine', expect one match with latest stable version",
		cmd:    "search repo alpine",
		golden: "output/search-multiple-stable-release.txt",
	}, {
		name:   "search for 'alpine', expect one match with newest development version",
		cmd:    "search repo alpine --devel",
		golden: "output/search-multiple-devel-release.txt",
	}, {
		name:   "search for 'alpine' with versions, expect three matches",
		cmd:    "search repo alpine --versions",
		golden: "output/search-multiple-versions.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.1.0",
		cmd:    "search repo alpine --version '>= 0.1, < 0.2'",
		golden: "output/search-constraint.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.1.0",
		cmd:    "search repo alpine --versions --version '>= 0.1, < 0.2'",
		golden: "output/search-versions-constraint.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.2.0",
		cmd:    "search repo alpine --version '>= 0.1'",
		golden: "output/search-constraint-single.txt",
	}, {
		name:   "search for 'alpine' with version constraint and --versions, expect two matches",
		cmd:    "search repo alpine --versions --version '>= 0.1'",
		golden: "output/search-multiple-versions-constraints.txt",
	}, {
		name:   "search for 'syzygy', expect no matches",
		cmd:    "search repo syzygy",
		golden: "output/search-not-found.txt",
	}, {
		name:      "search for 'syzygy' with --fail-on-no-result, expect failure for no results",
		cmd:       "search repo syzygy --fail-on-no-result",
		golden:    "output/search-not-found-error.txt",
		wantError: true,
	}, {name: "search for 'syzygy' with json output and --fail-on-no-result, expect failure for no results",
		cmd:       "search repo syzygy --output json --fail-on-no-result",
		golden:    "output/search-not-found-error.txt",
		wantError: true,
	}, {
		name:      "search for 'syzygy' with yaml output --fail-on-no-result, expect failure for no results",
		cmd:       "search repo syzygy --output yaml --fail-on-no-result",
		golden:    "output/search-not-found-error.txt",
		wantError: true,
	}, {
		name:   "search for 'alp[a-z]+', expect two matches",
		cmd:    "search repo alp[a-z]+ --regexp",
		golden: "output/search-regex.txt",
	}, {
		name:      "search for 'alp[', expect failure to compile regexp",
		cmd:       "search repo alp[ --regexp",
		wantError: true,
	}, {
		name:   "search for 'maria', expect valid json output",
		cmd:    "search repo maria --output json",
		golden: "output/search-output-json.txt",
	}, {
		name:   "search for 'alpine', expect valid yaml output",
		cmd:    "search repo alpine --output yaml",
		golden: "output/search-output-yaml.txt",
	}}

	settings.Debug = true
	defer func() { settings.Debug = false }()

	for i := range tests {
		tests[i].cmd += " --repository-config " + repoFile
		tests[i].cmd += " --repository-cache " + repoCache
	}
	runTestCmd(t, tests)
}

func TestSearchOCIRepositoriesCmd(t *testing.T) {
	// Initialize settings for registry client
	defer resetEnv()()
	settings.Debug = true
	defer func() { settings.Debug = false }()

	// Create a temporary directory for registry config
	tmpDir := t.TempDir()
	settings.RegistryConfig = filepath.Join(tmpDir, "config.json")

	// Mock OCI registry server
	mux := http.NewServeMux()

	// Mock tags endpoint
	mux.HandleFunc("/v2/test/chart/tags/list", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"test/chart","tags":["1.0.0","1.0.1","0.9.0"]}`)
	})

	// Define config blob data
	config1Data := []byte(`{"name":"test-chart","version":"1.0.0","appVersion":"2.0.0","description":"Test chart v1.0.0"}`)
	config2Data := []byte(`{"name":"test-chart","version":"1.0.1","appVersion":"2.0.1","description":"Test chart v1.0.1"}`)
	config3Data := []byte(`{"name":"test-chart","version":"0.9.0","appVersion":"1.9.0","description":"Test chart v0.9.0"}`)

	// These are the actual SHA256 digests of the config blob content
	config1Digest := "sha256:15f5a75b7de16679a895bb173e9668e466c0246a2de3ed81584145389fbabd2e"
	config2Digest := "sha256:6afde066d0fe3e0c4d09f10b59fa17687f7fdff5333cd33717d0ed1eb26d0bc6"
	config3Digest := "sha256:d64b81c7994b23bca85a62cad7ac300fed3dee7c3fee976ab12244e1ca1690a7"

	// Define manifest data (must include size for ORAS)
	manifest1Data := []byte(fmt.Sprintf(`{"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json","digest":"%s","size":%d}}`, config1Digest, len(config1Data)))
	manifest2Data := []byte(fmt.Sprintf(`{"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json","digest":"%s","size":%d}}`, config2Digest, len(config2Data)))
	manifest3Data := []byte(fmt.Sprintf(`{"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json","digest":"%s","size":%d}}`, config3Digest, len(config3Data)))

	// Calculate the actual SHA256 digests of the manifest content
	// Note: These need to be recalculated if manifest content changes
	manifest1Digest := "sha256:904b47f9a2b3548df25d33bb230a6eb6788d5f2bab3d8c54788b3be1e92208de"
	manifest2Digest := "sha256:3a49cb8ff737393d8b81aa5afa62c81328397a3a19d8b2b7341d1478ec3aa4f0"
	manifest3Digest := "sha256:b3977d49b98f0ad1c2f2a9145c6cd175b005d8742d0e71e170395e84beaee5a9"

	// Mock manifest HEAD endpoints
	mux.HandleFunc("/v2/test/chart/manifests/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", manifest1Digest)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest1Data)))
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/v2/test/chart/manifests/1.0.1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", manifest2Digest)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest2Data)))
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/v2/test/chart/manifests/0.9.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", manifest3Digest)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest3Data)))
			w.WriteHeader(http.StatusOK)
		}
	})

	// Mock manifest GET endpoints by digest
	mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", manifest1Digest), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", manifest1Digest)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest1Data)))
		w.Write(manifest1Data)
	})

	mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", manifest2Digest), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", manifest2Digest)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest2Data)))
		w.Write(manifest2Data)
	})

	mux.HandleFunc(fmt.Sprintf("/v2/test/chart/manifests/%s", manifest3Digest), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", manifest3Digest)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest3Data)))
		w.Write(manifest3Data)
	})

	// Mock config blobs
	mux.HandleFunc(fmt.Sprintf("/v2/test/chart/blobs/%s", config1Digest), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.cncf.helm.config.v1+json")
		w.Header().Set("Docker-Content-Digest", config1Digest)
		w.Write(config1Data)
	})

	mux.HandleFunc(fmt.Sprintf("/v2/test/chart/blobs/%s", config2Digest), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.cncf.helm.config.v1+json")
		w.Header().Set("Docker-Content-Digest", config2Digest)
		w.Write(config2Data)
	})

	mux.HandleFunc(fmt.Sprintf("/v2/test/chart/blobs/%s", config3Digest), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.cncf.helm.config.v1+json")
		w.Header().Set("Docker-Content-Digest", config3Digest)
		w.Write(config3Data)
	})

	// Add catch-all handler to log unhandled requests
	wrappedMux := http.NewServeMux()
	wrappedMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only log if we haven't handled it yet
		if r.URL.Path != "/" {
			t.Logf("Mock server received: %s %s", r.Method, r.URL.Path)
		}
		mux.ServeHTTP(w, r)
	})

	server := httptest.NewServer(wrappedMux)
	defer server.Close()

	// Extract host and port for test setup
	serverHost := server.URL[7:] // Remove http://

	tests := []cmdTestCase{{
		name:   "search OCI registry for latest version",
		cmd:    fmt.Sprintf("search repo oci://%s/test/chart --plain-http", serverHost),
		golden: "output/search-oci-single.txt",
	}, {
		name:   "search OCI registry for all versions",
		cmd:    fmt.Sprintf("search repo oci://%s/test/chart --versions --plain-http", serverHost),
		golden: "output/search-oci-versions.txt",
	}, {
		name:   "search OCI registry with version constraint",
		cmd:    fmt.Sprintf("search repo oci://%s/test/chart --version '>= 1.0.0' --versions --plain-http", serverHost),
		golden: "output/search-oci-constraint.txt",
	}}

	// Run tests with custom output processing to replace dynamic port
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			storage := storageFixture()
			_, out, err := executeActionCommandC(storage, tt.cmd)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got success with the following output:\n%s", out)
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no error, got: '%v'", err)
			}

			// Replace dynamic port with placeholder for comparison
			// Extract port from serverHost (e.g., "127.0.0.1:12345" -> "12345")
			parts := strings.Split(serverHost, ":")
			if len(parts) == 2 {
				port := parts[1]
				out = strings.ReplaceAll(out, ":"+port, ":<PORT>")
			}

			if tt.golden != "" {
				test.AssertGoldenString(t, out, tt.golden)
			}
		})
	}

	// Test rate limit error handling
	t.Run("search OCI registry with rate limit error", func(t *testing.T) {
		// Create a mock server that returns rate limit errors
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Mock server received: %s %s", r.Method, r.URL.Path)

			if r.URL.Path == "/v2/test/chart/tags/list" {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"name":"test/chart","tags":["1.0.0"]}`)
				return
			}

			// Return rate limit error for manifest requests
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"errors": [{"code": "TOOMANYREQUESTS", "message": "You have reached your pull rate limit"}]}`)
		}))
		defer ts.Close()

		u, _ := url.Parse(ts.URL)
		host := u.Host
		ref := fmt.Sprintf("oci://%s/test/chart", host)

		cmd := fmt.Sprintf("search repo %s --plain-http", ref)

		// Need to provide a storage configuration
		storage := storageFixture()
		_, _, err := executeActionCommandC(storage, cmd)

		if err == nil {
			t.Error("expected rate limit error, got nil")
		} else if !strings.Contains(err.Error(), "rate limit exceeded") {
			t.Errorf("expected rate limit error message, got: %v", err)
		}
	})
}

func TestSearchRepoOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "search repo")
}

func TestSearchRepoFileCompletion(t *testing.T) {
	checkFileCompletion(t, "search repo", true) // File completion may be useful when inputting a keyword
}
