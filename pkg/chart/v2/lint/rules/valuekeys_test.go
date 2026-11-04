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

package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindBooleanLikeKeys(t *testing.T) {
	for _, tt := range []struct {
		name string
		doc  string
		want []string
	}{
		{
			name: "plain keys that YAML 1.1 reads as booleans",
			// A Grafana dashboard pasted into values is the common way to hit this.
			doc:  "gridPos:\n  h: 8\n  w: 12\n  x: 0\n  y: 0\n",
			want: []string{"gridPos.y"},
		},
		{
			name: "inside a sequence",
			doc:  "panels:\n  - title: cpu\n    on: true\n",
			want: []string{"panels[0].on"},
		},
		{
			name: "every affected spelling",
			doc:  "a:\n  y: 1\n  Y: 1\n  yes: 1\n  no: 1\n  N: 1\n  off: 1\n  ON: 1\n  True: 1\n",
			want: []string{"a.y", "a.Y", "a.yes", "a.no", "a.N", "a.off", "a.ON", "a.True"},
		},
		{
			name: "quoted keys survive and are not reported",
			doc:  "a:\n  \"y\": 1\n  'on': 2\n",
			want: nil,
		},
		{
			name: "ordinary keys",
			doc:  "replicaCount: 1\nimage:\n  tag: latest\n",
			want: nil,
		},
		{
			name: "empty document",
			doc:  "",
			want: nil,
		},
		{
			name: "unparsable document is left to the values rule",
			doc:  "a:\n\t- broken\n",
			want: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			found, err := findBooleanLikeKeys([]byte(tt.doc))
			require.NoError(t, err)
			paths := make([]string, 0, len(found))
			for _, k := range found {
				paths = append(paths, k.path)
			}
			assert.ElementsMatch(t, tt.want, paths)
		})
	}
}

func TestValidateNoBooleanLikeKeys(t *testing.T) {
	dir := t.TempDir()

	affected := filepath.Join(dir, "values.yaml")
	require.NoError(t, os.WriteFile(affected, []byte("gridPos:\n  x: 0\n  y: 0\n"), 0o644))
	err := validateNoBooleanLikeKeys(affected)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gridPos.y")
	assert.Contains(t, err.Error(), "line 3")

	clean := filepath.Join(dir, "clean.yaml")
	require.NoError(t, os.WriteFile(clean, []byte("gridPos:\n  x: 0\n  \"y\": 0\n"), 0o644))
	assert.NoError(t, validateNoBooleanLikeKeys(clean))

	// A missing file is reported by validateValuesFileExistence, not here.
	assert.NoError(t, validateNoBooleanLikeKeys(filepath.Join(dir, "nope.yaml")))
}
