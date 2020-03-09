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
	srv, err := repotest.NewTempServer("testdata/testcharts/*.tgz*")
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
