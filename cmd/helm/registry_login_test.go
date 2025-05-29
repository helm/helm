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

func TestRegistryLoginFileCompletion(t *testing.T) {
	checkFileCompletion(t, "registry login", false)
}

func TestRegistryLogin(t *testing.T) {
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

	// Creating an empty json file to use as an empty registry configuration file. Used for
	// testing the handling of this situation.
	tmpDir := t.TempDir()
	emptyFile, err := os.Create(filepath.Join(tmpDir, "empty.json"))
	if err != nil {
		t.Fatal(err)
	}
	emptyFile.Close()

	tests := []struct {
		name         string
		cmd          string
		wantError    bool
		wantErrorMsg string
	}{
		{
			name: "Normal login",
			cmd:  fmt.Sprintf("registry login %s --username %s --password %s", ociSrv.RegistryURL, ociSrv.TestUsername, ociSrv.TestPassword),
		},
		{
			name:      "Failed login",
			cmd:       fmt.Sprintf("registry login %s --username foo --password bar", ociSrv.RegistryURL),
			wantError: true,
		},
		{
			name: "Normal login with empty registry file",
			cmd:  fmt.Sprintf("registry login %s --username %s --password %s --registry-config %s", ociSrv.RegistryURL, ociSrv.TestUsername, ociSrv.TestPassword, emptyFile.Name()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := executeActionCommand(tt.cmd)
			if err != nil {
				if tt.wantError {
					if tt.wantErrorMsg != "" && tt.wantErrorMsg == err.Error() {
						t.Fatalf("Actual error %s, not equal to expected error %s", err, tt.wantErrorMsg)
					}
					return
				}
				t.Fatalf("%q reported error: %s", tt.name, err)
			}
		})
	}
}
