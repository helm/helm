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

// Package e2e contains end-to-end tests that exercise a real helm binary
// against real external systems (an OCI registry, and optionally a Kubernetes
// cluster).
//
// These tests are excluded from the normal build by the "e2e" build tag and
// are never run by `make test`. Run them with `make test-e2e` after exporting
// the configuration described in the package README.
package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Environment variables used to configure the end-to-end tests.
const (
	// envRegistry is the registry namespace charts are pushed to, without a
	// scheme, e.g. "ghcr.io/example-org" or "localhost:5000/charts".
	envRegistry = "HELM_E2E_REGISTRY"
	// envUsername is the username used to authenticate to envRegistry.
	envUsername = "HELM_E2E_USERNAME"
	// envPassword is the password or token used to authenticate to envRegistry.
	envPassword = "HELM_E2E_PASSWORD"
	// envHelmBin points at a prebuilt helm binary. When unset, one is built
	// from the working tree.
	envHelmBin = "HELM_E2E_BIN"
	// envPlainHTTP requests plain HTTP for registries without TLS, which is
	// useful when testing against a local registry.
	envPlainHTTP = "HELM_E2E_PLAIN_HTTP"
	// envRepo is the repository path under the registry that charts are
	// pushed to. Helm appends the chart name, so this is a prefix shared by
	// every fixture.
	envRepo = "HELM_E2E_REPO"
	// envIsolate appends a per-run suffix to the repository path. It is off by
	// default so that the set of repositories a run touches is stable and can
	// be created ahead of time on registries that do not create repositories
	// on push.
	envIsolate = "HELM_E2E_ISOLATE"
	// envKubernetes opts in to the tests that install charts into a real
	// Kubernetes cluster using the ambient kubeconfig.
	envKubernetes = "HELM_E2E_KUBERNETES"
	// envNamespace is the namespace the Kubernetes tests install into.
	envNamespace = "HELM_E2E_NAMESPACE"
)

// harness carries the resolved configuration shared by the end-to-end tests.
type harness struct {
	// helmBin is an absolute path to the helm binary under test.
	helmBin string
	// registry is the registry namespace charts are pushed to.
	registry string
	// username and password authenticate to the registry. They are only ever
	// passed to helm over stdin and are never logged.
	username string
	password string
	// plainHTTP is true when the registry should be reached over HTTP.
	plainHTTP bool
	// repoBase is the repository path charts are pushed under.
	repoBase string
	// isolate appends runID to repoBase.
	isolate bool
	// configPath is the registry credential file used for this run.
	configPath string
	// runID isolates the repositories written by a single test run so that
	// concurrent or repeated runs do not collide.
	runID string
}

// newHarness resolves the end-to-end configuration, failing the test when a
// required value is missing. Because the "e2e" build tag is an explicit opt-in,
// a missing setting is a configuration error rather than a reason to skip.
func newHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{
		registry:  requireEnv(t, envRegistry),
		username:  requireEnv(t, envUsername),
		password:  requireEnv(t, envPassword),
		plainHTTP: os.Getenv(envPlainHTTP) != "",
		repoBase:  envOr(envRepo, "helm-e2e"),
		isolate:   os.Getenv(envIsolate) != "",
		helmBin:   helmBinary(t),
		runID:     runID(t),
	}
	return h
}

// requireEnv returns the value of the named environment variable, failing the
// test with an actionable message when it is unset.
func requireEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Fatalf("%s must be set to run the end-to-end tests; see test/e2e/README.md", name)
	}
	return v
}

// helmBinary returns the helm binary to exercise, building one from the
// working tree when HELM_E2E_BIN is not set.
func helmBinary(t *testing.T) string {
	t.Helper()

	if bin := os.Getenv(envHelmBin); bin != "" {
		abs, err := filepath.Abs(bin)
		if err != nil {
			t.Fatalf("resolving %s=%q: %v", envHelmBin, bin, err)
		}
		if _, err := os.Stat(abs); err != nil {
			t.Fatalf("%s=%q is not usable: %v", envHelmBin, bin, err)
		}
		return abs
	}

	bin := filepath.Join(t.TempDir(), "helm")
	build := exec.Command("go", "build", "-o", bin, "helm.sh/helm/v4/cmd/helm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building helm: %v\n%s", err, out)
	}
	return bin
}

// runID returns a short random identifier unique to this test run.
func runID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generating run id: %v", err)
	}
	return hex.EncodeToString(b)
}

// registryHost returns the host portion of the configured registry, which is
// what `helm registry login` expects.
func (h *harness) registryHost() string {
	host, _, _ := strings.Cut(h.registry, "/")
	return host
}

// repo returns the repository path charts are pushed to. Helm appends the
// chart name, so a run touches one repository per fixture.
//
// The path is stable by default. Re-pushing a fixture overwrites the same tag
// with byte-identical content, so concurrent runs do not interfere, the set of
// repositories stays bounded, and registries that require repositories to
// exist before a push (ECR) can have them created ahead of time. Set
// HELM_E2E_ISOLATE to give a run its own repositories instead.
func (h *harness) repo() string {
	if h.isolate {
		return fmt.Sprintf("%s/%s-%s", h.registry, h.repoBase, h.runID)
	}
	return fmt.Sprintf("%s/%s", h.registry, h.repoBase)
}

// envOr returns the value of the named environment variable, or def when it is
// unset.
func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// ref returns a full OCI reference for a chart within an isolated repository.
func (h *harness) ref(repo, chart, version string) string {
	if version == "" {
		return fmt.Sprintf("oci://%s/%s", repo, chart)
	}
	return fmt.Sprintf("oci://%s/%s:%s", repo, chart, version)
}

// plainHTTPCommands are the helm subcommands that accept --plain-http. The
// flag is only appended for these so that the harness can still run commands
// such as `status` and `uninstall` against a plain HTTP registry.
var plainHTTPCommands = map[string]bool{"push": true, "pull": true, "install": true, "upgrade": true}

// helm runs the helm binary with the given arguments and returns its combined
// output. Arguments are logged, so callers must never pass a credential.
func (h *harness) helm(t *testing.T, args ...string) (string, error) {
	t.Helper()
	if h.plainHTTP && len(args) > 0 && plainHTTPCommands[args[0]] {
		args = append(args, "--plain-http")
	}
	t.Logf("helm %s", strings.Join(args, " "))
	cmd := exec.Command(h.helmBin, args...)
	cmd.Env = append(os.Environ(), "HELM_REGISTRY_CONFIG="+h.registryConfig(t))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// mustHelm runs helm and fails the test if the command does not succeed.
func (h *harness) mustHelm(t *testing.T, args ...string) string {
	t.Helper()
	out, err := h.helm(t, args...)
	if err != nil {
		t.Fatalf("helm %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// registryConfig returns the path to this run's registry credential file. The
// file lives under a per-test temporary directory so credentials never leak
// into the developer's real helm configuration.
func (h *harness) registryConfig(t *testing.T) string {
	t.Helper()
	if h.configPath == "" {
		h.configPath = filepath.Join(t.TempDir(), "registry-config.json")
	}
	return h.configPath
}

// login authenticates to the configured registry. The password is written to
// helm's stdin rather than passed as a flag so it cannot appear in process
// listings or test output.
func (h *harness) login(t *testing.T, password string) (string, error) {
	t.Helper()
	args := []string{"registry", "login", h.registryHost(), "--username", h.username, "--password-stdin"}
	if h.plainHTTP {
		args = append(args, "--plain-http")
	}
	t.Logf("helm %s", strings.Join(args, " "))
	cmd := exec.Command(h.helmBin, args...)
	cmd.Env = append(os.Environ(), "HELM_REGISTRY_CONFIG="+h.registryConfig(t))
	cmd.Stdin = strings.NewReader(password)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// mustLogin authenticates with the configured credential and arranges for a
// logout once the test completes.
func (h *harness) mustLogin(t *testing.T) {
	t.Helper()
	out, err := h.login(t, h.password)
	if err != nil {
		t.Fatalf("registry login to %s failed: %v\n%s", h.registryHost(), err, out)
	}
	t.Cleanup(func() {
		if out, err := h.helm(t, "registry", "logout", h.registryHost()); err != nil {
			t.Logf("registry logout failed (ignored): %v\n%s", err, out)
		}
	})
}

// chart returns the path to a chart archive in this package's testdata.
func chart(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", "testcharts", name))
	if err != nil {
		t.Fatalf("resolving chart %q: %v", name, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("chart %q is missing: %v", name, err)
	}
	return path
}
