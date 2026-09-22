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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"helm.sh/helm/v4/pkg/cli"
)

// TestOCIRegistryPushPull pushes chart archives to the configured OCI registry
// and pulls them back, verifying that the artifact round-trips intact.
func TestOCIRegistryPushPull(t *testing.T) {
	h := newHarness(t)
	h.mustLogin(t)

	repo := h.repo()
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

			// Verify the round-trip actually returned the chart we pushed,
			// not merely some artifact under the same tag.
			assertChartMatches(t, chart(t, tt.chart), pullDir, tt.expectPullDir)
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
		// An anonymous registry, such as a default registry:2 deployment,
		// accepts any credential. There is no authentication to exercise, so
		// skip rather than report a failure the registry cannot produce.
		t.Skipf("Skipping invalid credential test: %s accepts unauthenticated access", h.registryHost())
	}
	if !isAuthError(out) {
		t.Fatalf("expected an authentication failure, got a different error:\n%s", out)
	}

	// Seed the credential store directly with the same wrong password, so the
	// push reaches the registry and is rejected by it rather than failing
	// locally for want of any credential at all.
	writeRegistryCredential(t, h.registryConfig(t), h.registryHost(), h.username, "invalid-"+h.runID)

	out, err = h.helm(t, "push", chart(t, "test-0.1.0.tgz"), "oci://"+h.repo())
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

	// A caller-supplied namespace is left alone; one we create for the run is
	// removed again so repeated runs do not accumulate namespaces in the
	// cluster.
	namespace := os.Getenv(envNamespace)
	if namespace == "" {
		namespace = "helm-e2e-" + h.runID
		t.Cleanup(func() { deleteNamespace(t, namespace) })
	}
	repo := h.repo()

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

			// Scope the release name to this run. A caller-supplied namespace
			// may already hold releases, and a fixed name could collide with
			// one, which cleanup would then uninstall.
			releaseName := fmt.Sprintf("%s-%s", tt.releaseName, h.runID)

			t.Cleanup(func() {
				if out, err := h.helm(t, "uninstall", releaseName, "--namespace", namespace, "--ignore-not-found", "--wait"); err != nil {
					t.Errorf("uninstalling %s failed: %v\n%s", releaseName, err, out)
				}
			})

			h.mustHelm(t, "install", releaseName, h.ref(repo, tt.chartName, ""),
				"--version", tt.chartVersion,
				"--namespace", namespace,
				"--create-namespace",
				"--wait",
				"--timeout", "2m",
			)

			// Read the release back from cluster storage to confirm the
			// install really reached the API server.
			out := h.mustHelm(t, "status", releaseName, "--namespace", namespace, "--output", "json")
			if !strings.Contains(out, `"status":"deployed"`) {
				t.Errorf("expected release %s to be deployed, got:\n%s", releaseName, out)
			}
		})
	}
}

// assertChartMatches verifies that what was pulled into pullDir is the same
// chart as the fixture at src. Existence and file-vs-directory checks alone
// would pass if the registry served a different artifact under the same tag.
//
// A chart pulled without --untar is the pushed archive verbatim, so it is
// compared byte for byte. With --untar, helm expands the archive, so every
// regular file in the source archive is compared against its extracted
// counterpart.
func assertChartMatches(t *testing.T, src, pullDir string, untarred bool) {
	t.Helper()

	if !untarred {
		assertFilesEqual(t, src, filepath.Join(pullDir, filepath.Base(src)))
		return
	}

	want := archiveFiles(t, src)
	if len(want) == 0 {
		t.Fatalf("fixture %s contains no files", src)
	}
	for name, content := range want {
		extracted := filepath.Join(pullDir, filepath.FromSlash(name))
		got, err := os.ReadFile(extracted)
		if err != nil {
			t.Errorf("expected extracted file %s: %v", extracted, err)
			continue
		}
		if !bytes.Equal(got, content) {
			t.Errorf("extracted file %s does not match the pushed chart", name)
		}
	}
}

// assertFilesEqual compares two files byte for byte.
func assertFilesEqual(t *testing.T, want, got string) {
	t.Helper()
	wantBytes, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("reading %s: %v", want, err)
	}
	gotBytes, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading %s: %v", got, err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		t.Errorf("pulled chart %s does not match the pushed chart %s (%d vs %d bytes)",
			got, want, len(gotBytes), len(wantBytes))
	}
}

// archiveFiles returns the regular files in a gzipped tar archive, keyed by
// their slash-separated path within the archive.
func archiveFiles(t *testing.T, path string) map[string][]byte {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("reading %s as gzip: %v", path, err)
	}
	defer gz.Close()

	files := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading %s as tar: %v", path, err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("reading %s from %s: %v", hdr.Name, path, err)
		}
		files[hdr.Name] = content
	}
	return files
}

// deleteNamespace removes a namespace created by the test run. It is best
// effort in the sense that it reports a failure rather than aborting the test,
// but it does not silently leave the namespace behind.
func deleteNamespace(t *testing.T, namespace string) {
	t.Helper()

	// Resolve the cluster exactly the way the helm subprocesses do. They read
	// their Kubernetes configuration from the environment (HELM_KUBECONTEXT,
	// HELM_KUBEAPISERVER, HELM_KUBETOKEN, HELM_KUBECAFILE and friends), so
	// building this client any other way risks installing releases into one
	// cluster and sending the namespace delete to another. Such a delete
	// returns NotFound, which would look like success while leaking the
	// namespace. Going through helm's own settings keeps the two in step
	// without having to mirror each variable here.
	cfg, err := cli.New().RESTClientGetter().ToRESTConfig()
	if err != nil {
		t.Errorf("building kube client config to delete namespace %s: %v", namespace, err)
		return
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Errorf("building kube client to delete namespace %s: %v", namespace, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Logf("deleting namespace %s", namespace)
	if err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		t.Errorf("deleting namespace %s: %v", namespace, err)
		return
	}

	// Deletion is asynchronous. Wait for the namespace to actually go away so
	// that a namespace wedged in Terminating is reported rather than quietly
	// accumulating in the cluster.
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Errorf("waiting for namespace %s to be deleted: %v", namespace, err)
	}
}
