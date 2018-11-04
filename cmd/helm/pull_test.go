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
	"regexp"
	"strings"
	"testing"

	"k8s.io/helm/pkg/repo/repotest"
)

func TestPullCmd(t *testing.T) {
	defer resetEnv()()

	hh := testHelmHome(t)
	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()

	// all flags will get "--home=TMDIR -d outdir" appended.
	tests := []struct {
		name         string
		args         []string
		wantError    bool
		failExpect   string
		expectFile   string
		expectDir    bool
		expectVerify bool
	}{
		{
			name:       "Basic chart fetch",
			args:       []string{"test/signtest"},
			expectFile: "./signtest-0.1.0.tgz",
		},
		{
			name:       "Chart fetch with version",
			args:       []string{"test/signtest --version=0.1.0"},
			expectFile: "./signtest-0.1.0.tgz",
		},
		{
			name:       "Fail chart fetch with non-existent version",
			args:       []string{"test/signtest --version=99.1.0"},
			wantError:  true,
			failExpect: "no such chart",
		},
		{
			name:       "Fail fetching non-existent chart",
			args:       []string{"test/nosuchthing"},
			failExpect: "Failed to fetch",
			wantError:  true,
		},
		{
			name:         "Fetch and verify",
			args:         []string{"test/signtest --verify --keyring testdata/helm-test-key.pub"},
			expectFile:   "./signtest-0.1.0.tgz",
			expectVerify: true,
		},
		{
			name:       "Fetch and fail verify",
			args:       []string{"test/reqtest --verify --keyring testdata/helm-test-key.pub"},
			failExpect: "Failed to fetch provenance",
			wantError:  true,
		},
		{
			name:       "Fetch and untar",
			args:       []string{"test/signtest --untar --untardir signtest"},
			expectFile: "./signtest",
			expectDir:  true,
		},
		{
			name:         "Fetch, verify, untar",
			args:         []string{"test/signtest --verify --keyring=testdata/helm-test-key.pub --untar --untardir signtest"},
			expectFile:   "./signtest",
			expectDir:    true,
			expectVerify: true,
		},
		{
			name:       "Chart fetch using repo URL",
			expectFile: "./signtest-0.1.0.tgz",
			args:       []string{"signtest --repo", srv.URL()},
		},
		{
			name:       "Fail fetching non-existent chart on repo URL",
			args:       []string{"someChart --repo", srv.URL()},
			failExpect: "Failed to fetch chart",
			wantError:  true,
		},
		{
			name:       "Specific version chart fetch using repo URL",
			expectFile: "./signtest-0.1.0.tgz",
			args:       []string{"signtest --version=0.1.0 --repo", srv.URL()},
		},
		{
			name:       "Specific version chart fetch using repo URL",
			args:       []string{"signtest --version=0.2.0 --repo", srv.URL()},
			failExpect: "Failed to fetch chart version",
			wantError:  true,
		},
	}

	if _, err := srv.CopyCharts("testdata/testcharts/*.tgz*"); err != nil {
		t.Fatal(err)
	}
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		outdir := hh.Path("testout")
		os.RemoveAll(outdir)
		os.Mkdir(outdir, 0755)

		cmd := strings.Join(append(tt.args, "-d", outdir, "--home", hh.String()), " ")
		out, err := executeCommand(nil, "fetch "+cmd)
		if err != nil {
			if tt.wantError {
				continue
			}
			t.Errorf("%q reported error: %s", tt.name, err)
			continue
		}

		if tt.expectVerify {
			pointerAddressPattern := "0[xX][A-Fa-f0-9]+"
			sha256Pattern := "[A-Fa-f0-9]{64}"
			verificationRegex := regexp.MustCompile(
				fmt.Sprintf("Verification: &{%s sha256:%s signtest-0.1.0.tgz}\n", pointerAddressPattern, sha256Pattern))
			if !verificationRegex.MatchString(out) {
				t.Errorf("%q: expected match for regex %s, got %s", tt.name, verificationRegex, out)
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
	}
}
