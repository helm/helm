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
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestMutualtlsPull(t *testing.T) {
	srv, err := repotest.NewTempmtlsServerWithCleanup(t, "testdata/testcharts/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stopmtls()

	ociSrv, err := repotest.NewOCImtlsServer(t, srv.Rootmtls())
	if err != nil {
		t.Fatal(err)
	}
	ociSrv.Run(t)

	if err := srv.LinkIndicesmtls(); err != nil {
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
			name:       "Fetch OCI Chart",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart --version 0.1.0 --ca-file ../../testdata/rootca.crt --cert-file ../../testdata/rootca.crt --key-file ../../testdata/rootca.key --mtls-enabled", ociSrv.RegistryURL),
			expectFile: "./oci-dependent-chart-0.1.0.tgz",
		},
		{
			name:       "Fail fetching non-existent OCI chart with mutual tls enabled",
			args:       fmt.Sprintf("oci://%s/u/ocitestuser/nosuchthing --version 0.1.0 --mtls-enabled", ociSrv.RegistryURL),
			failExpect: "Failed to fetch",
			wantError:  true,
		},
		{
			name:         "Fail fetching OCI chart without version specified with mutual tls enabled",
			args:         fmt.Sprintf("oci://%s/u/ocitestuser/nosuchthing --mtls-enabled", ociSrv.RegistryURL),
			wantErrorMsg: "Error: --version flag is explicitly required for OCI registries",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdir := srv.Rootmtls()
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
