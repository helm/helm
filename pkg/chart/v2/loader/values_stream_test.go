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

package loader_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v3loader "helm.sh/helm/v4/internal/chart/v3/loader"
	v2loader "helm.sh/helm/v4/pkg/chart/v2/loader"
)

func TestLoadValuesYAMLStream(t *testing.T) {
	tests := []struct {
		name string
		data string
		want map[string]any
	}{
		{
			name: "inline empty mapping",
			data: "--- {}\n",
			want: map[string]any{},
		},
		{
			name: "inline mapping without trailing newline",
			data: "--- {message: hello}",
			want: map[string]any{"message": "hello"},
		},
		{
			name: "merge inline documents",
			data: "--- {service: {image: old, port: 80}, keep: original}\n--- {}\n--- {service: {image: new}}\n",
			want: map[string]any{
				"service": map[string]any{"image": "new", "port": float64(80)},
				"keep":    "original",
			},
		},
		{
			name: "document end and directive",
			data: "--- {first: value}\n...\n%YAML 1.1\n--- {second: value}\n",
			want: map[string]any{"first": "value", "second": "value"},
		},
		{
			name: "empty and comment documents",
			data: "# before\n---\n# empty\n--- {} # inline\n---\nmessage: hello\n---\n",
			want: map[string]any{"message": "hello"},
		},
		{
			name: "scalar and key compatibility",
			data: "--- {enabled: yes, disabled: off, count: 2, fraction: 1.5, quoted: 'yes', date: 2026-01-15, 42: number-key}\n",
			want: map[string]any{
				"enabled":  true,
				"disabled": false,
				"count":    float64(2),
				"fraction": float64(1.5),
				"quoted":   "yes",
				"date":     "2026-01-15",
				"42":       "number-key",
			},
		},
		{
			name: "aliases and merge keys",
			data: "---\ndefaults: &defaults\n  image: example:v1\nservice:\n  <<: *defaults\n  port: 80\n",
			want: map[string]any{
				"defaults": map[string]any{"image": "example:v1"},
				"service":  map[string]any{"image": "example:v1", "port": float64(80)},
			},
		},
	}

	loaders := map[string]func(io.Reader) (map[string]any, error){
		"chart v2": v2loader.LoadValues,
		"chart v3": v3loader.LoadValues,
	}
	for name, loadValues := range loaders {
		t.Run(name, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					values, err := loadValues(strings.NewReader(test.data))
					require.NoError(t, err)
					assert.Equal(t, test.want, values)
				})
			}
		})
	}
}

func TestLoadValuesYAMLStreamErrors(t *testing.T) {
	loaders := map[string]func(io.Reader) (map[string]any, error){
		"chart v2": v2loader.LoadValues,
		"chart v3": v3loader.LoadValues,
	}
	for name, loadValues := range loaders {
		t.Run(name, func(t *testing.T) {
			for _, data := range []string{
				"--- {broken: [}\n",
				"--- {valid: first}\n--- {broken: [}\n",
				"--- [not, a, mapping]\n",
				"--- scalar\n",
			} {
				t.Run(data, func(t *testing.T) {
					values, err := loadValues(strings.NewReader(data))
					require.Error(t, err)
					assert.Nil(t, values)
				})
			}
		})
	}
}
