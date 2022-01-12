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
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestShowPreReleaseChart(t *testing.T) {
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/testcharts/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		args        string
		flags       string
		fail        bool
		expectedErr string
	}{
		{
			name:        "show pre-release chart",
			args:        "test/pre-release-chart",
			fail:        true,
			expectedErr: "failed to download \"test/pre-release-chart\"",
		},
		{
			name:        "show pre-release chart",
			args:        "test/pre-release-chart",
			fail:        true,
			flags:       "--version 1.0.0",
			expectedErr: "failed to download \"test/pre-release-chart\" at version \"1.0.0\"",
		},
		{
			name:  "show pre-release chart with 'devel' flag",
			args:  "test/pre-release-chart",
			flags: "--devel",
			fail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdir := srv.Root()
			cmd := fmt.Sprintf("show all '%s' %s --repository-config %s --repository-cache %s",
				tt.args,
				tt.flags,
				filepath.Join(outdir, "repositories.yaml"),
				outdir,
			)
			//_, out, err := executeActionCommand(cmd)
			_, _, err := executeActionCommand(cmd)
			if err != nil {
				if tt.fail {
					if !strings.Contains(err.Error(), tt.expectedErr) {
						t.Errorf("%q expected error: %s, got: %s", tt.name, tt.expectedErr, err.Error())
					}
					return
				}
				t.Errorf("%q reported error: %s", tt.name, err)
			}
		})
	}
}

func TestShowVersionCompletion(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	tests := []cmdTestCase{{
		name:   "completion for show version flag",
		cmd:    fmt.Sprintf("%s __complete show chart testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for show version flag, no filter",
		cmd:    fmt.Sprintf("%s __complete show chart testing/alpine --version 0.3", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for show version flag too few args",
		cmd:    fmt.Sprintf("%s __complete show chart --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for show version flag too many args",
		cmd:    fmt.Sprintf("%s __complete show chart testing/alpine badarg --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for show version flag invalid chart",
		cmd:    fmt.Sprintf("%s __complete show chart invalid/invalid --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for show version flag with all",
		cmd:    fmt.Sprintf("%s __complete show all testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for show version flag with readme",
		cmd:    fmt.Sprintf("%s __complete show readme testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for show version flag with values",
		cmd:    fmt.Sprintf("%s __complete show values testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestShowFileCompletion(t *testing.T) {
	checkFileCompletion(t, "show", false)
}

func TestShowAllFileCompletion(t *testing.T) {
	checkFileCompletion(t, "show all", true)
}

func TestShowChartFileCompletion(t *testing.T) {
	checkFileCompletion(t, "show chart", true)
}

func TestShowReadmeFileCompletion(t *testing.T) {
	checkFileCompletion(t, "show readme", true)
}

func TestShowValuesFileCompletion(t *testing.T) {
	checkFileCompletion(t, "show values", true)
}

func TestShowCRDsFileCompletion(t *testing.T) {
	checkFileCompletion(t, "show crds", true)
}
