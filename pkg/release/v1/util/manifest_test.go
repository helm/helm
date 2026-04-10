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

package util // import "helm.sh/helm/v4/pkg/release/v1/util"

import (
	"reflect"
	"testing"
)

func TestSplitManifests(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "single doc with leading separator and whitespace",
			input: `

---
apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo,
  annotations:
    "helm.sh/hook": test
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo,
  annotations:
    "helm.sh/hook": test
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command
`,
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "whitespace only",
			input:    "  \n\n  \n",
			expected: map[string]string{},
		},
		{
			name:  "whitespace-only doc after separator is skipped",
			input: "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\n---\n  \n",
			expected: map[string]string{
				"manifest-0": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1",
			},
		},
		{
			name: "single doc no separator",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			},
		},
		{
			name: "two docs with proper separator",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
			},
		},

		// Block scalar chomping tests: verify that trailing newlines
		// are preserved through SplitManifests for single-doc inputs.

		// | (clip)
		{
			name: "block scalar clip (|) with 0 trailing newlines",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello`,
			},
		},
		{
			name: "block scalar clip (|) with 1 trailing newline",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello
`,
			},
		},
		{
			name: "block scalar clip (|) with 2 trailing newlines",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello

`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello

`,
			},
		},

		// |- (strip)
		{
			name: "block scalar strip (|-) with 0 trailing newlines",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello`,
			},
		},
		{
			name: "block scalar strip (|-) with 1 trailing newline",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello
`,
			},
		},
		{
			name: "block scalar strip (|-) with 2 trailing newlines",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello

`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello

`,
			},
		},

		// |+ (keep)
		{
			name: "block scalar keep (|+) with 0 trailing newlines",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello`,
			},
		},
		{
			name: "block scalar keep (|+) with 1 trailing newline",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello
`,
			},
		},
		{
			name: "block scalar keep (|+) with 2 trailing newlines",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello

`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello

`,
			},
		},

		// Multi-doc with block scalars: the regex consumes \s*\n before ---,
		// so trailing newlines from non-last docs are stripped.
		{
			name: "multi-doc block scalar clip (|) before separator",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
`,
			},
		},
		{
			name: "multi-doc block scalar keep (|+) with 2 trailing newlines before separator",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello


---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
`,
			},
		},

		// **Note for Chart API v3**: The following tests exercise the lenient
		// regex that splits `---apiVersion` back into separate documents.
		// In Chart API v3, these inputs should return an _ERROR_ instead.
		// See the comment on the SplitManifests function for more details.
		{
			name: "leading glued separator (---apiVersion)",
			input: `
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
`,
			},
		},
		{
			name: "mid-content glued separator (---apiVersion)",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
			},
		},
		{
			name: "multiple glued separators",
			input: `
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2`,
				"manifest-2": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
`,
			},
		},
		{
			name: "mixed glued and proper separators",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
`,
			expected: map[string]string{
				"manifest-0": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2`,
				"manifest-2": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitManifests(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SplitManifests() =\n%v\nwant:\n%v", result, tt.expected)
			}
		})
	}
}
