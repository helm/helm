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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/action"
)

func TestTemplateCmdYAMLStreamValues(t *testing.T) {
	tests := []struct {
		name     string
		defaults string
		values   string
		want     map[string]string
	}{
		{
			name:     "inline empty defaults and values",
			defaults: "--- {}\n",
			values:   "--- {}\n",
			want:     map[string]string{"message": "default", "count": "1"},
		},
		{
			name:     "merge inline values with defaults",
			defaults: "--- {message: original, count: 2}\n",
			values:   "--- {message: first}\n--- {}\n--- {message: final}\n",
			want:     map[string]string{"message": "final", "count": "2"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chartDir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(chartDir, "templates"), 0o755))
			files := map[string]string{
				"Chart.yaml":  "apiVersion: v2\nname: yaml-stream\nversion: 0.1.0\n",
				"values.yaml": test.defaults,
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: yaml-stream
data:
  message: {{ .Values.message | default "default" | quote }}
  count: {{ .Values.count | default 1 | quote }}
`,
			}
			for name, contents := range files {
				require.NoError(t, os.WriteFile(filepath.Join(chartDir, name), []byte(contents), 0o644))
			}
			valuesPath := filepath.Join(t.TempDir(), "values.yaml")
			require.NoError(t, os.WriteFile(valuesPath, []byte(test.values), 0o644))

			var output bytes.Buffer
			command := newTemplateCmd(action.NewConfiguration(), &output)
			command.SetArgs([]string{"example", chartDir, "--values", valuesPath})
			command.SetOut(&output)
			command.SetErr(&output)
			require.NoError(t, command.Execute(), output.String())

			var manifest struct {
				Data map[string]string `json:"data"`
			}
			require.NoError(t, yaml.Unmarshal(output.Bytes(), &manifest))
			assert.Equal(t, test.want, manifest.Data)
		})
	}
}
