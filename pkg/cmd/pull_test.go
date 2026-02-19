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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestPullCmd(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
	)
	defer srv.Stop()

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	if err != nil {
		t.Fatal(err)
	}
	ociSrv.Run(t)

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	helmTestKeyOut := "Signed by: Helm Testing (This key should only be used for testing. DO NOT TRUST.) <helm-testing@helm.sh>\n" +
		"Using Key With Fingerprint: 5E615389B53CA37F0EE60BD3843BBF981FC18762\n" +
		"Chart Hash Verified: "

	// all flags will get "-d outdir" appended.
	tests := []struct {
		name         string
		args         string
		existFile    string
		existDir     string
		wantError    bool
		wantErrorMsg string
		failExpect   string
		expectFile   string
		expectDir    bool
		expectVerify bool
		expectSha    string
	}{
		{
			name:       "Basic chart fetch",
			args:       "test/signtest",
			expectFile: "./signtest-0.1.0.tgz",
		},
		{
			name:       "Chart fetch with version",
			args:       "test/signtest --version=0.1.0",
			expectFile: "./signtest-0.1.0.tgz",
		},
		{
			name:       "Fail chart fetch with non-existent version",
			args:       "test/signtest --version=99.1.0",
			wantError:  true,
			failExpect: "no such chart",
		},
		{
			name:       "Fail fetching non-existent chart",
			args:       "test/nosuchthing",
			failExpect: "Failed to fetch",
			wantError:  true,
		},
		{
			name:         "Fetch and verify",
			args:         "test/signtest --verify --keyring testdata/helm-test-key.pub",
			expectFile:   "./signtest-0.1.0.tgz",
			expectVerify: true,
			expectSha:    "sha256:e5ef611620fb97704d8751c16bab17fedb68883bfb0edc76f78a70e9173f9b55",
		},
		{
			name:       "Fetch and fail verify",
			args:       "test/reqtest --verify --keyring testdata/helm-test-key.pub",
			failExpect: "Failed to fetch provenance",
			wantError:  true,
		},
		{
			name:       "Fetch and untar",
			args:       "test/signtest --untar --untardir signtest",
			expectFile: "./signtest",
			expectDir:  true,
		},
		{
			name:         "Fetch untar when file with same name existed",
			args:         "test/test1 --untar --untardir test1",
			existFile:    "test1/test1",
			wantError:    true,
			wantErrorMsg: fmt.Sprintf("failed to untar: a file or directory with the name %s already exists", filepath.Join(srv.Root(), "test1", "test1")),
		},
		{
			name:         "Fetch untar when dir with same name existed",
			args:         "test/test --untar --untardir test2",
			existDir:     "test2/test",
			wantError:    true,
			wantErrorMsg: fmt.Sprintf("failed to untar: a file or directory with the name %s already exists", filepath.Join(srv.Root(), "test2", "test")),
		},
		{
			name:         "Fetch, verify, untar",
			args:         "test/signtest --verify --keyring=testdata/helm-test-key.pub --untar --untardir signtest2",
			expectFile:   "./signtest2",
			expectDir:    true,
			expectVerify: true,
			expectSha:    "sha256:e5ef611620fb97704d8751c16bab17fedb68883bfb0edc76f78a70e9173f9b55",
		},
		{
			name:       "Chart fetch using repo URL",
			expectFile: "./signtest-0.1.0.tgz",
			args:       "signtest --repo " + srv.URL(),
		},
		{
			name:       "Fail fetching non-existent chart on repo URL",
			args:       "someChart --repo " + srv.URL(),
			failExpect: "Failed to fetch chart",
			wantError:  true,
		},
		{
			name:       "Specific version chart fetch using repo URL",
			expectFile: "./signtest-0.1.0.tgz",
			args:       "signtest --version=0.1.0 --repo " + srv.URL(),
		},
		{
			name:       "Specific version chart fetch using repo URL",
			args:       "signtest --version=0.2.0 --repo " + srv.URL(),
			failExpect: "Failed to fetch chart version",
			wantError:  true,
		},
		{
			name:       "Chart fetch using repo URL with untardir",
			args:       "signtest --version=0.1.0 --untar --untardir repo-url-test --repo " + srv.URL(),
			expectFile: "./signtest",
			expectDir:  true,
		},
		{
			name:       "Chart fetch using repo URL with untardir and previous pull",
			args:       "signtest --version=0.1.0 --untar --untardir repo-url-test --repo " + srv.URL(),
			failExpect: "failed to untar",
			wantError:  true,
		},
		{
			name:       "Fetch OCI Chart",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart --version 0.1.0", ociSrv.RegistryURL),
			expectFile: "./oci-dependent-chart-0.1.0.tgz",
		},
		{
			name:       "Fetch OCI Chart with untar",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart --version 0.1.0 --untar", ociSrv.RegistryURL),
			expectFile: "./oci-dependent-chart",
			expectDir:  true,
		},
		{
			name:       "Fetch OCI Chart with untar and untardir",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart --version 0.1.0 --untar --untardir ocitest2", ociSrv.RegistryURL),
			expectFile: "./ocitest2",
			expectDir:  true,
		},
		{
			name:         "OCI Fetch untar when dir with same name existed",
			args:         fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart --version 0.1.0 --untar --untardir ocitest2", ociSrv.RegistryURL),
			existDir:     "ocitest2/oci-dependent-chart",
			wantError:    true,
			wantErrorMsg: fmt.Sprintf("failed to untar: a file or directory with the name %s already exists", filepath.Join(srv.Root(), "ocitest2", "oci-dependent-chart")),
		},
		{
			name:       "Fail fetching non-existent OCI chart",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/nosuchthing --version 0.1.0", ociSrv.RegistryURL),
			failExpect: "Failed to fetch",
			wantError:  true,
		},
		{
			name:      "Fail fetching OCI chart without version specified",
			args:      fmt.Sprintf("oci://%s/u/ocitestuser/nosuchthing", ociSrv.RegistryURL),
			wantError: true,
		},
		{
			name:       "Fetching OCI chart without version option specified",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart:0.1.0", ociSrv.RegistryURL),
			expectFile: "./oci-dependent-chart-0.1.0.tgz",
		},
		{
			name:       "Fetching OCI chart with version specified",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart:0.1.0 --version 0.1.0", ociSrv.RegistryURL),
			expectFile: "./oci-dependent-chart-0.1.0.tgz",
		},
		{
			name:         "Fail fetching OCI chart with version mismatch",
			args:         fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart:0.2.0 --version 0.1.0", ociSrv.RegistryURL),
			wantErrorMsg: "chart reference and version mismatch: 0.1.0 is not 0.2.0",
			wantError:    true,
		},
	}

	contentCache := t.TempDir()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdir := srv.Root()
			cmd := fmt.Sprintf("fetch %s -d '%s' --repository-config %s --repository-cache %s --registry-config %s --content-cache %s --plain-http",
				tt.args,
				outdir,
				filepath.Join(outdir, "repositories.yaml"),
				outdir,
				filepath.Join(outdir, "config.json"),
				contentCache,
			)
			// Create file or Dir before helm pull --untar, see: https://github.com/helm/helm/issues/7182
			if tt.existFile != "" {
				file := filepath.Join(outdir, tt.existFile)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				_, err := os.Create(file)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tt.existDir != "" {
				file := filepath.Join(outdir, tt.existDir)
				err := os.MkdirAll(file, 0755)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, out, err := executeActionCommand(cmd)
			if err != nil {
				if tt.wantError {
					if tt.wantErrorMsg != "" && tt.wantErrorMsg != err.Error() {
						t.Fatalf("Actual error '%s', not equal to expected error '%s'", err, tt.wantErrorMsg)
					}
					return
				}
				t.Fatalf("%q reported error: %s", tt.name, err)
			}

			if tt.expectVerify {
				outString := helmTestKeyOut + tt.expectSha + "\n"
				if out != outString {
					t.Errorf("%q: expected verification output %q, got %q", tt.name, outString, out)
				}

			}

			ef := filepath.Join(outdir, tt.expectFile)
			fi, err := os.Stat(ef)
			if err != nil {
				t.Errorf("%q: expected a file at %s. %s", tt.name, ef, err)
			}
			if fi.IsDir() != tt.expectDir {
				t.Errorf("%q: expected directory=%t, but it's not.", tt.name, tt.expectDir)
			}
		})
	}
}

// runPullTests is a helper function to run pull command tests with common logic
func runPullTests(t *testing.T, tests []struct {
	name         string
	args         string
	existFile    string
	existDir     string
	wantError    bool
	wantErrorMsg string
	expectFile   string
	expectDir    bool
}, outdir string, additionalFlags string) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := fmt.Sprintf("pull %s -d '%s' --repository-config %s --repository-cache %s --registry-config %s %s",
				tt.args,
				outdir,
				filepath.Join(outdir, "repositories.yaml"),
				outdir,
				filepath.Join(outdir, "config.json"),
				additionalFlags,
			)
			// Create file or Dir before helm pull --untar, see: https://github.com/helm/helm/issues/7182
			if tt.existFile != "" {
				file := filepath.Join(outdir, tt.existFile)
				_, err := os.Create(file)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tt.existDir != "" {
				file := filepath.Join(outdir, tt.existDir)
				err := os.MkdirAll(file, 0755)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, _, err := executeActionCommand(cmd)
			if tt.wantError && err == nil {
				t.Fatalf("%q: expected error but got none", tt.name)
			}
			if err != nil {
				if tt.wantError {
					if tt.wantErrorMsg != "" && tt.wantErrorMsg != err.Error() {
						t.Fatalf("Actual error '%s', not equal to expected error '%s'", err, tt.wantErrorMsg)
					}
					return
				}
				t.Fatalf("%q reported error: %s", tt.name, err)
			}

			ef := filepath.Join(outdir, tt.expectFile)
			fi, err := os.Stat(ef)
			if err != nil {
				t.Errorf("%q: expected a file at %s. %s", tt.name, ef, err)
			}
			if fi.IsDir() != tt.expectDir {
				t.Errorf("%q: expected directory=%t, but it's not.", tt.name, tt.expectDir)
			}
		})
	}
}

// buildOCIURL is a helper function to build OCI URLs with credentials
func buildOCIURL(registryURL, chartName, version, username, password string) string {
	baseURL := fmt.Sprintf("oci://%s/u/ocitestuser/%s", registryURL, chartName)
	if version != "" {
		baseURL += fmt.Sprintf(" --version %s", version)
	}
	if username != "" && password != "" {
		baseURL += fmt.Sprintf(" --username %s --password %s", username, password)
	}
	return baseURL
}

func TestPullWithCredentialsCmd(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
		repotest.WithMiddleware(repotest.BasicAuthMiddleware(t)),
	)
	defer srv.Stop()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.Dir(srv.Root())).ServeHTTP(w, r)
	}))
	defer srv2.Close()

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	// all flags will get "-d outdir" appended.
	tests := []struct {
		name         string
		args         string
		existFile    string
		existDir     string
		wantError    bool
		wantErrorMsg string
		expectFile   string
		expectDir    bool
	}{
		{
			name:       "Chart fetch using repo URL",
			expectFile: "./signtest-0.1.0.tgz",
			args:       "signtest --repo " + srv.URL() + " --username username --password password",
		},
		{
			name:      "Fail fetching non-existent chart on repo URL",
			args:      "someChart --repo " + srv.URL() + " --username username --password password",
			wantError: true,
		},
		{
			name:       "Specific version chart fetch using repo URL",
			expectFile: "./signtest-0.1.0.tgz",
			args:       "signtest --version=0.1.0 --repo " + srv.URL() + " --username username --password password",
		},
		{
			name:      "Specific version chart fetch using repo URL",
			args:      "signtest --version=0.2.0 --repo " + srv.URL() + " --username username --password password",
			wantError: true,
		},
		{
			name:       "Chart located on different domain with credentials passed",
			args:       "reqtest --repo " + srv2.URL + " --username username --password password --pass-credentials",
			expectFile: "./reqtest-0.1.0.tgz",
		},
	}

	runPullTests(t, tests, srv.Root(), "")
}

func TestPullVersionCompletion(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	tests := []cmdTestCase{{
		name:   "completion for pull version flag",
		cmd:    fmt.Sprintf("%s __complete pull testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for pull version flag, no filter",
		cmd:    fmt.Sprintf("%s __complete pull testing/alpine --version 0.3", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for pull version flag too few args",
		cmd:    fmt.Sprintf("%s __complete pull --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for pull version flag too many args",
		cmd:    fmt.Sprintf("%s __complete pull testing/alpine badarg --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for pull version flag invalid chart",
		cmd:    fmt.Sprintf("%s __complete pull invalid/invalid --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestPullWithCredentialsCmdOCIRegistry(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
	)
	defer srv.Stop()

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	if err != nil {
		t.Fatal(err)
	}
	ociSrv.Run(t)

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	// all flags will get "-d outdir" appended.
	tests := []struct {
		name         string
		args         string
		existFile    string
		existDir     string
		wantError    bool
		wantErrorMsg string
		expectFile   string
		expectDir    bool
	}{
		{
			name:       "OCI Chart fetch with credentials",
			args:       buildOCIURL(ociSrv.RegistryURL, "oci-dependent-chart", "0.1.0", ociSrv.TestUsername, ociSrv.TestPassword),
			expectFile: "./oci-dependent-chart-0.1.0.tgz",
		},
		{
			name:       "OCI Chart fetch with credentials and untar",
			args:       buildOCIURL(ociSrv.RegistryURL, "oci-dependent-chart", "0.1.0", ociSrv.TestUsername, ociSrv.TestPassword) + " --untar",
			expectFile: "./oci-dependent-chart",
			expectDir:  true,
		},
		{
			name:       "OCI Chart fetch with credentials and untardir",
			args:       buildOCIURL(ociSrv.RegistryURL, "oci-dependent-chart", "0.1.0", ociSrv.TestUsername, ociSrv.TestPassword) + " --untar --untardir ocitest-credentials",
			expectFile: "./ocitest-credentials",
			expectDir:  true,
		},
		{
			name:      "Fail fetching OCI chart with wrong credentials",
			args:      buildOCIURL(ociSrv.RegistryURL, "oci-dependent-chart", "0.1.0", "wronguser", "wrongpass"),
			wantError: true,
		},
		{
			name:      "Fail fetching non-existent OCI chart with credentials",
			args:      buildOCIURL(ociSrv.RegistryURL, "nosuchthing", "0.1.0", ociSrv.TestUsername, ociSrv.TestPassword),
			wantError: true,
		},
		{
			name:      "Fail fetching OCI chart without version specified",
			args:      buildOCIURL(ociSrv.RegistryURL, "nosuchthing", "", ociSrv.TestUsername, ociSrv.TestPassword),
			wantError: true,
		},
	}

	runPullTests(t, tests, srv.Root(), "--plain-http")
}

func TestPullFileCompletion(t *testing.T) {
	checkFileCompletion(t, "pull", false)
	checkFileCompletion(t, "pull repo/chart", false)
}

// TestPullOCIWithTagAndDigest tests pulling an OCI chart with both tag and digest specified.
// This is a regression test for https://github.com/helm/helm/issues/31600
func TestPullOCIWithTagAndDigest(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
	)
	defer srv.Stop()

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	if err != nil {
		t.Fatal(err)
	}
	result := ociSrv.RunWithReturn(t)

	contentCache := t.TempDir()
	outdir := t.TempDir()

	// Test: pull with tag and digest (the fixed bug from issue #31600)
	// Previously this failed with "encoding/hex: invalid byte: U+0073 's'"
	ref := fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart:0.1.0@%s",
		ociSrv.RegistryURL, result.PushedChart.Manifest.Digest)

	cmd := fmt.Sprintf("pull %s -d '%s' --registry-config %s --content-cache %s --plain-http",
		ref,
		outdir,
		filepath.Join(srv.Root(), "config.json"),
		contentCache,
	)

	_, _, err = executeActionCommand(cmd)
	if err != nil {
		t.Fatalf("pull with tag+digest failed: %v", err)
	}

	// Verify the file was downloaded
	// When digest is present, the filename uses the digest format (e.g. chart@sha256-hex.tgz)
	expectedFile := filepath.Join(outdir, "oci-dependent-chart-0.1.0.tgz")
	if _, err := os.Stat(expectedFile); err != nil {
		// Try the digest-based filename; parse algorithm:hex to avoid fixed-offset assumptions
		algorithm, digestPart, ok := strings.Cut(result.PushedChart.Manifest.Digest, ":")
		if !ok {
			t.Fatalf("digest must be in algorithm:hex format, got %q", result.PushedChart.Manifest.Digest)
		}
		expectedFile = filepath.Join(outdir, fmt.Sprintf("oci-dependent-chart@%s-%s.tgz", algorithm, digestPart))
		if _, err := os.Stat(expectedFile); err != nil {
			t.Errorf("expected chart file not found: %v", err)
		}
	}
}
