//go:build e2e

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

package e2e

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOCIRegistryPushPull pushes chart archives to the configured OCI registry
// and pulls them back, verifying that the artifact round-trips intact.
func TestOCIRegistryPushPull(t *testing.T) {
	h := newHarness(t)
	h.mustLogin(t)

	repo := h.repo("push-pull")
	contentCache := t.TempDir()

	tests := []struct {
		name           string
		chart          string
		chartName      string
		chartVersion   string
		pullArgs       []string
		expectPullFile string
		expectPullDir  bool
	}{
		{
			name:           "basic chart",
			chart:          "test-0.1.0.tgz",
			chartName:      "test",
			chartVersion:   "0.1.0",
			expectPullFile: "test-0.1.0.tgz",
		},
		{
			name:           "chart pulled with untar",
			chart:          "compressedchart-0.1.0.tgz",
			chartName:      "compressedchart",
			chartVersion:   "0.1.0",
			pullArgs:       []string{"--untar"},
			expectPullFile: "compressedchart",
			expectPullDir:  true,
		},
		{
			name:           "chart name with hyphens",
			chart:          "compressedchart-with-hyphens-0.1.0.tgz",
			chartName:      "compressedchart-with-hyphens",
			chartVersion:   "0.1.0",
			expectPullFile: "compressedchart-with-hyphens-0.1.0.tgz",
		},
		{
			name:           "chart with unicode description",
			chart:          "unicode-chart-0.1.0.tgz",
			chartName:      "unicode-chart",
			chartVersion:   "0.1.0",
			expectPullFile: "unicode-chart-0.1.0.tgz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pullDir := t.TempDir()

			h.mustHelm(t, "push", chart(t, tt.chart), "oci://"+repo)

			pullArgs := append([]string{
				"pull", h.ref(repo, tt.chartName, tt.chartVersion),
				"--destination", pullDir,
				"--content-cache", contentCache,
			}, tt.pullArgs...)
			h.mustHelm(t, pullArgs...)

			pulled := filepath.Join(pullDir, tt.expectPullFile)
			fi, err := os.Stat(pulled)
			if err != nil {
				t.Fatalf("expected pulled chart at %s: %v", pulled, err)
			}
			if fi.IsDir() != tt.expectPullDir {
				t.Errorf("expected directory=%t, got directory=%t", tt.expectPullDir, fi.IsDir())
			}
		})
	}
}

// TestOCIRegistryInvalidCredentials verifies that the registry rejects a bad
// credential with an authentication error, rather than any error at all.
func TestOCIRegistryInvalidCredentials(t *testing.T) {
	h := newHarness(t)

	// Log in with a deliberately wrong password against the same registry and
	// namespace the successful tests use, so the only variable is the
	// credential itself.
	out, err := h.login(t, "invalid-"+h.runID)
	if err == nil {
		t.Fatal("expected registry login to fail with an invalid credential, but it succeeded")
	}
	if !isAuthError(out) {
		t.Fatalf("expected an authentication failure, got a different error:\n%s", out)
	}

	// Seed the credential store directly with the same wrong password, so the
	// push reaches the registry and is rejected by it rather than failing
	// locally for want of any credential at all.
	writeRegistryCredential(t, h.registryConfig(t), h.registryHost(), h.username, "invalid-"+h.runID)

	out, err = h.helm(t, "push", chart(t, "test-0.1.0.tgz"), "oci://"+h.repo("auth-failure"))
	if err == nil {
		t.Fatal("expected push to fail with an invalid credential, but it succeeded")
	}
	if !isAuthError(out) {
		t.Fatalf("expected an authentication failure, got a different error:\n%s", out)
	}
}

// writeRegistryCredential writes a docker-style credential file so a test can
// present a specific credential to the registry without going through
// `helm registry login`, which refuses to store one the registry rejects.
func writeRegistryCredential(t *testing.T, path, host, username, password string) {
	t.Helper()
	cfg := struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}{
		Auths: map[string]struct {
			Auth string `json:"auth"`
		}{
			host: {Auth: base64.StdEncoding.EncodeToString([]byte(username + ":" + password))},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("encoding registry credential: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("writing registry credential to %s: %v", path, err)
	}
}

// isAuthError reports whether the output describes an authentication or
// authorization failure, as opposed to a network, DNS, or naming error that
// would otherwise make the test pass for the wrong reason.
func isAuthError(out string) bool {
	out = strings.ToLower(out)
	for _, marker := range []string{
		"401",
		"403",
		"unauthorized",
		"denied",
		"authentication required",
		"invalid username/password",
	} {
		if strings.Contains(out, marker) {
			return true
		}
	}
	return false
}

// TestOCIRegistryInstallToKubernetes exercises the full flow of pushing a
// chart to an OCI registry and installing it into a real Kubernetes cluster
// using the ambient kubeconfig. It is opt-in because, unlike the registry
// tests, it mutates a cluster.
func TestOCIRegistryInstallToKubernetes(t *testing.T) {
	if os.Getenv(envKubernetes) == "" {
		t.Skipf("Skipping Kubernetes end-to-end test: set %s to run it against the current kubecontext", envKubernetes)
	}

	h := newHarness(t)
	h.mustLogin(t)

	namespace := os.Getenv(envNamespace)
	if namespace == "" {
		namespace = "helm-e2e-" + h.runID
	}
	repo := h.repo("install")

	tests := []struct {
		name         string
		chart        string
		chartName    string
		chartVersion string
		releaseName  string
	}{
		{
			name:         "basic chart",
			chart:        "test-0.1.0.tgz",
			chartName:    "test",
			chartVersion: "0.1.0",
			releaseName:  "test-release",
		},
		{
			name:         "chart with unicode description",
			chart:        "unicode-chart-0.1.0.tgz",
			chartName:    "unicode-chart",
			chartVersion: "0.1.0",
			releaseName:  "unicode-release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.mustHelm(t, "push", chart(t, tt.chart), "oci://"+repo)

			t.Cleanup(func() {
				if out, err := h.helm(t, "uninstall", tt.releaseName, "--namespace", namespace, "--ignore-not-found", "--wait"); err != nil {
					t.Errorf("uninstalling %s failed: %v\n%s", tt.releaseName, err, out)
				}
			})

			h.mustHelm(t, "install", tt.releaseName, h.ref(repo, tt.chartName, ""),
				"--version", tt.chartVersion,
				"--namespace", namespace,
				"--create-namespace",
				"--wait",
				"--timeout", "2m",
			)

			// Read the release back from cluster storage to confirm the
			// install really reached the API server.
			out := h.mustHelm(t, "status", tt.releaseName, "--namespace", namespace, "--output", "json")
			if !strings.Contains(out, `"status":"deployed"`) {
				t.Errorf("expected release %s to be deployed, got:\n%s", tt.releaseName, out)
			}
		})
	}
}
