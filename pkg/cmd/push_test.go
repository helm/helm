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
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestPushCmd(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
	)
	defer srv.Stop()

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	require.NoError(t, err)
	ociSrv.Run(t)

	ref := ociSrv.RegistryURL + "/u/ocitestuser/compressedchart:0.1.0"
	digestPattern := `^sha256:[0-9a-f]{64}$`

	tests := []struct {
		name   string
		format string
		check  func(t *testing.T, out string)
	}{
		{
			name: "push with default table output",
			check: func(t *testing.T, out string) {
				t.Helper()
				assert.Contains(t, out, fmt.Sprintf("Pushed: %s\n", ref))
				assert.Regexp(t, `Digest: sha256:[0-9a-f]{64}\n`, out)
			},
		},
		{
			name:   "push with table output",
			format: "table",
			check: func(t *testing.T, out string) {
				t.Helper()
				assert.Contains(t, out, fmt.Sprintf("Pushed: %s\n", ref))
				assert.Regexp(t, `Digest: sha256:[0-9a-f]{64}\n`, out)
			},
		},
		{
			name:   "push with json output",
			format: "json",
			check: func(t *testing.T, out string) {
				t.Helper()
				result := map[string]string{}
				require.NoError(t, json.Unmarshal([]byte(out), &result), "expected pure JSON output, got %q", out)
				assert.Equal(t, ref, result["ref"])
				assert.Regexp(t, digestPattern, result["digest"])
			},
		},
		{
			name:   "push with yaml output",
			format: "yaml",
			check: func(t *testing.T, out string) {
				t.Helper()
				result := map[string]string{}
				require.NoError(t, yaml.Unmarshal([]byte(out), &result), "expected pure YAML output, got %q", out)
				assert.Equal(t, ref, result["ref"])
				assert.Regexp(t, digestPattern, result["digest"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := fmt.Sprintf("push testdata/testcharts/compressedchart-0.1.0.tgz oci://%s/u/ocitestuser --registry-config %s --plain-http",
				ociSrv.RegistryURL,
				filepath.Join(srv.Root(), "config.json"),
			)
			if tt.format != "" {
				cmd += " --output " + tt.format
			}
			_, out, err := executeActionCommand(cmd)
			require.NoError(t, err)
			tt.check(t, out)
		})
	}
}

func TestPushOutputCompletion(t *testing.T) {
	runTestCmd(t, []cmdTestCase{{
		name:   "completion for output flag of push",
		cmd:    "__complete push --output ''",
		golden: "output/output-comp.txt",
	}})
}

func TestPushFileCompletion(t *testing.T) {
	checkFileCompletion(t, "push", true)
	checkFileCompletion(t, "push package.tgz", false)
	checkFileCompletion(t, "push package.tgz oci://localhost:5000", false)
}
