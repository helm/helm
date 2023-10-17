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

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestPullCmd(t *testing.T) {
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/testcharts/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
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
			existFile:    "test1",
			wantError:    true,
			wantErrorMsg: fmt.Sprintf("failed to untar: a file or directory with the name %s already exists", filepath.Join(srv.Root(), "test1")),
		},
		{
			name:         "Fetch untar when dir with same name existed",
			args:         "test/test2 --untar --untardir test2",
			existDir:     "test2",
			wantError:    true,
			wantErrorMsg: fmt.Sprintf("failed to untar: a file or directory with the name %s already exists", filepath.Join(srv.Root(), "test2")),
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
			args:         fmt.Sprintf("oci-test-chart oci://%s/u/ocitestuser/oci-dependent-chart --version 0.1.0 --untar --untardir ocitest2 --untar --untardir ocitest2", ociSrv.RegistryURL),
			wantError:    true,
			wantErrorMsg: fmt.Sprintf("failed to untar: a file or directory with the name %s already exists", filepath.Join(srv.Root(), "ocitest2")),
		},
		{
			name:       "Fail fetching non-existent OCI chart",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/nosuchthing --version 0.1.0", ociSrv.RegistryURL),
			failExpect: "Failed to fetch",
			wantError:  true,
		},
		{
			name:         "Fail fetching OCI chart without version specified",
			args:         fmt.Sprintf("oci://%s/u/ocitestuser/nosuchthing", ociSrv.RegistryURL),
			wantErrorMsg: "Error: --version flag is explicitly required for OCI registries",
			wantError:    true,
		},
		{
			name:         "Fail fetching OCI chart without version specified",
			args:         fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart:0.1.0", ociSrv.RegistryURL),
			wantErrorMsg: "Error: --version flag is explicitly required for OCI registries",
			wantError:    true,
		},
		{
			name:      "Fail fetching OCI chart without version specified",
			args:      fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart:0.1.0 --version 0.1.0", ociSrv.RegistryURL),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdir := srv.Root()
			cmd := fmt.Sprintf("fetch %s -d '%s' --repository-config %s --repository-cache %s --registry-config %s",
				tt.args,
				outdir,
				filepath.Join(outdir, "repositories.yaml"),
				outdir,
				filepath.Join(outdir, "config.json"),
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
				err := os.Mkdir(file, 0755)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, out, err := executeActionCommand(cmd)
			if err != nil {
				if tt.wantError {
					if tt.wantErrorMsg != "" && tt.wantErrorMsg == err.Error() {
						t.Fatalf("Actual error %s, not equal to expected error %s", err, tt.wantErrorMsg)
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

func TestPullWithCredentialsCmd(t *testing.T) {
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/testcharts/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	srv.WithMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "username" || password != "password" {
			t.Errorf("Expected request to use basic auth and for username == 'username' and password == 'password', got '%v', '%s', '%s'", ok, username, password)
		}
	}))

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdir := srv.Root()
			cmd := fmt.Sprintf("pull %s -d '%s' --repository-config %s --repository-cache %s --registry-config %s",
				tt.args,
				outdir,
				filepath.Join(outdir, "repositories.yaml"),
				outdir,
				filepath.Join(outdir, "config.json"),
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
				err := os.Mkdir(file, 0755)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, _, err := executeActionCommand(cmd)
			if err != nil {
				if tt.wantError {
					if tt.wantErrorMsg != "" && tt.wantErrorMsg == err.Error() {
						t.Fatalf("Actual error %s, not equal to expected error %s", err, tt.wantErrorMsg)
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

func TestPullFileCompletion(t *testing.T) {
	checkFileCompletion(t, "pull", false)
	checkFileCompletion(t, "pull repo/chart", false)
}
