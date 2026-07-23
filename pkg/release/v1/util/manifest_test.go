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

package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
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
				"manifest-0": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\n",
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
  name: cm1
`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
			},
		},

		// Block scalar chomping indicator tests using | (clip), |- (strip), and |+ (keep)
		// inputs with 0, 1, and 2 trailing newlines after the block content.
		// Note: the emitter may normalize the output chomping indicator when the
		// trailing newline count makes another indicator equivalent for the result.

		// | (clip) input — clips trailing newlines to exactly one, though with
		// 0 trailing newlines the emitted output may normalize to |-.
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

		// Multi-doc with block scalars: the separator regex preserves trailing
		// newlines from non-last documents.
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
    hello
`,
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
    hello


`,
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
  name: cm1
`,
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
  name: cm1
`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
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
  name: cm1
`,
				"manifest-1": `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
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
			assert.Equal(t, tt.expected, result, "SplitManifests() =\n%v\nwant:\n%v", result, tt.expected)
		})
	}
}

func TestStripHelmInternalAnnotations(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expected          string
		mustNotContain    []string
		mustContain       []string
		mustEqualVerbatim bool
		mustParseOutput   bool
	}{
		{
			name: "strips multi-slash key, preserves siblings",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-a
  annotations:
    helm.sh/depends-on/resource-groups: '["foo"]'
    helm.sh/resource-group: app
    other.example.com/keep: "yes"
data:
  k: v
`,
			mustNotContain: []string{"helm.sh/depends-on/resource-groups"},
			mustContain:    []string{"helm.sh/resource-group", "other.example.com/keep", "name: cm-a"},
		},
		{
			name: "leaves dangling empty annotations key when sole annotation stripped",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-b
  annotations:
    helm.sh/depends-on/resource-groups: '["foo"]'
data:
  k: v
`,
			mustNotContain: []string{"helm.sh/depends-on/resource-groups"},
			mustContain:    []string{"name: cm-b", "data:"},
		},
		{
			name: "no annotations leaves doc unchanged",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-c
data:
  k: v
`,
			mustEqualVerbatim: true,
		},
		{
			name: "non-internal annotations preserved",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-d
  annotations:
    helm.sh/resource-group: app
data:
  k: v
`,
			mustEqualVerbatim: true,
		},
		{
			name:              "non-kubernetes yaml passes through",
			input:             "foo: bar\nbaz: 1\n",
			mustEqualVerbatim: true,
		},
		{
			name:              "invalid yaml passes through",
			input:             ":\n\tnot valid yaml at all: [",
			mustEqualVerbatim: true,
		},
		{
			name: "data block scalar look-alike survives",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-rules
data:
  rules.txt: |
    helm.sh/depends-on/resource-groups: ["a"]
    second line
`,
			mustEqualVerbatim: true,
		},
		{
			name: "annotation in metadata AND look-alike in data",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-mixed
  annotations:
    helm.sh/depends-on/resource-groups: '["db"]'
    helm.sh/resource-group: app
data:
  rules.txt: |
    helm.sh/depends-on/resource-groups: '["db"]'
    second line
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-mixed
  annotations:
    helm.sh/resource-group: app
data:
  rules.txt: |
    helm.sh/depends-on/resource-groups: '["db"]'
    second line
`,
		},
		{
			name: "multi-line annotation value stripped whole",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-multiline
  annotations:
    helm.sh/depends-on/resource-groups: >-
      ["databases",
      "cache"]
    keep.example.com/x: "y"
data:
  k: v
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-multiline
  annotations:
    keep.example.com/x: "y"
data:
  k: v
`,
		},
		{
			name: "folded annotation value with internal blank line stripped whole",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-blank
  annotations:
    helm.sh/depends-on/resource-groups: >-
      ["a",

      "b"]
    keep.example.com/x: "y"
data:
  k: v
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-blank
  annotations:
    keep.example.com/x: "y"
data:
  k: v
`,
			mustParseOutput: true,
		},
		{
			name: "literal annotation value with internal blank line stripped whole",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-literal-blank
  annotations:
    helm.sh/depends-on/resource-groups: |
      ["a",

      "b"]
    keep.example.com/x: "y"
data:
  script: |
    preserve

    me
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-literal-blank
  annotations:
    keep.example.com/x: "y"
data:
  script: |
    preserve

    me
`,
			mustParseOutput: true,
		},
		{
			name: "value containing '#' handled",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-hash
  annotations:
    helm.sh/depends-on/resource-groups: '["a#b"]'
    keep.example.com/x: "y"
data:
  k: v
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-hash
  annotations:
    keep.example.com/x: "y"
data:
  k: v
`,
		},
		{
			name: "multi-doc stream: only affected doc changes",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-clean
data:
  k: v
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-annotated
  annotations:
    helm.sh/depends-on/resource-groups: '["db"]'
    keep.example.com/x: "y"
data:
  k: v
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-data
data:
  rules.txt: |
    helm.sh/depends-on/resource-groups: ["db"]
    keep
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-clean
data:
  k: v
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-annotated
  annotations:
    keep.example.com/x: "y"
data:
  k: v
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-data
data:
  rules.txt: |
    helm.sh/depends-on/resource-groups: ["db"]
    keep
`,
		},
		{
			name:              "byte-identity fast path odd formatting",
			input:             "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-fast  \n  # comment about annotations\n\ndata:\n  k: v  ",
			mustEqualVerbatim: true,
		},
		{
			name:              "byte-identity fast path CRLF",
			input:             "apiVersion: v1\r\nkind: ConfigMap\r\nmetadata:\r\n  name: cm-crlf\r\ndata:\r\n  k: v\r\n",
			mustEqualVerbatim: true,
		},
		{
			name: "quoted key form stripped",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-quoted
  annotations:
    "helm.sh/depends-on/resource-groups": '["a"]'
    keep.example.com/x: "y"
data:
  k: v
`,
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-quoted
  annotations:
    keep.example.com/x: "y"
data:
  k: v
`,
		},
		{
			name: "separator inside block scalar does not corrupt",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-separator
data:
  script: |
---
    still byte-identical
`,
			mustEqualVerbatim: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHelmInternalAnnotations(tt.input)
			if tt.mustParseOutput {
				if err := yaml.Unmarshal([]byte(got), map[string]any{}); err != nil {
					t.Errorf("expected output to parse as yaml, got error: %v\n%s", err, got)
				}
			}
			if tt.expected != "" {
				if got != tt.expected {
					t.Errorf("expected exact output:\n%s\ngot:\n%s", tt.expected, got)
				}
				return
			}
			if tt.mustEqualVerbatim {
				if got != tt.input {
					t.Errorf("expected verbatim passthrough, got:\n%s", got)
				}
				return
			}
			for _, s := range tt.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output unexpectedly contains %q:\n%s", s, got)
				}
			}
			for _, s := range tt.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output missing expected %q:\n%s", s, got)
				}
			}
		})
	}
}
