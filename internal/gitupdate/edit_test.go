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

package gitupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyText(t *testing.T) {
	tests := []struct {
		name      string
		original  string
		options   EditOptions
		expected  string
		errorText string
	}{
		{
			name:     "overwrite",
			original: "old\n",
			options:  textOptions(MethodOverwrite, "new\n"),
			expected: "new\n",
		},
		{
			name:     "literal replacement",
			original: "replicas: 2\n",
			options: EditOptions{
				Method:          MethodReplace,
				Content:         []byte("replicas: 3"),
				ContentProvided: true,
				LiteralMatch:    "replicas: 2",
				ExpectedMatches: 1,
			},
			expected: "replicas: 3\n",
		},
		{
			name:     "insert after",
			original: "first\nlast\n",
			options: EditOptions{
				Method:          MethodInsertAfter,
				Content:         []byte("middle\n"),
				ContentProvided: true,
				LiteralMatch:    "first\n",
				ExpectedMatches: 1,
			},
			expected: "first\nmiddle\nlast\n",
		},
		{
			name:     "regular expression capture expansion",
			original: "image: app:v1\n",
			options: EditOptions{
				Method:          MethodReplace,
				Content:         []byte("${1}v2"),
				ContentProvided: true,
				RegexMatch:      `(image: app:)v1`,
				ExpectedMatches: 1,
				Expand:          true,
			},
			expected: "image: app:v2\n",
		},
		{
			name:     "replace all",
			original: "dev dev dev",
			options: EditOptions{
				Method:          MethodReplace,
				Content:         []byte("prod"),
				ContentProvided: true,
				LiteralMatch:    "dev",
				ExpectedMatches: 3,
				AllMatches:      true,
			},
			expected: "prod prod prod",
		},
		{
			name:     "safe count rejects an ambiguous selector",
			original: "dev dev",
			options: EditOptions{
				Method:          MethodReplace,
				Content:         []byte("prod"),
				ContentProvided: true,
				LiteralMatch:    "dev",
				ExpectedMatches: 1,
			},
			errorText: "selector matched 2 times; expected 1",
		},
		{
			name:     "selector must match",
			original: "dev",
			options: EditOptions{
				Method:          MethodReplace,
				Content:         []byte("prod"),
				ContentProvided: true,
				LiteralMatch:    "staging",
				ExpectedMatches: 0,
			},
			errorText: "selector did not match",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := Apply([]byte(test.original), test.options)
			if test.errorText != "" {
				require.ErrorContains(t, err, test.errorText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, string(actual))
		})
	}
}

func TestApplyYAML(t *testing.T) {
	tests := []struct {
		name     string
		original string
		options  EditOptions
		expected string
	}{
		{
			name: "set nested value and preserve comment",
			original: `image:
  repository: example/app
  tag: v1 # deployed version
`,
			options: EditOptions{
				Method:          MethodYAMLSet,
				Content:         []byte("v2"),
				ContentProvided: true,
				Pointer:         "/image/tag",
			},
			expected: `image:
  repository: example/app
  tag: v2 # deployed version
`,
		},
		{
			name: "create mapping path",
			original: `image: {}
`,
			options: EditOptions{
				Method:          MethodYAMLSet,
				Content:         []byte("v2"),
				ContentProvided: true,
				Pointer:         "/image/tag",
				CreatePath:      true,
			},
			expected: `image: {tag: v2}
`,
		},
		{
			name: "deep merge",
			original: `image:
  repository: example/app
resources:
  limits:
    cpu: 100m
`,
			options: EditOptions{
				Method: MethodYAMLMerge,
				Content: []byte(`limits:
  memory: 128Mi
requests:
  cpu: 50m
`),
				ContentProvided: true,
				Pointer:         "/resources",
			},
			expected: `image:
  repository: example/app
resources:
  limits:
    cpu: 100m
    memory: 128Mi
  requests:
    cpu: 50m
`,
		},
		{
			name: "delete",
			original: `image:
  repository: example/app
  tag: v1
`,
			options: EditOptions{
				Method:  MethodYAMLDelete,
				Pointer: "/image/tag",
			},
			expected: `image:
  repository: example/app
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := Apply([]byte(test.original), test.options)
			require.NoError(t, err)
			assert.Equal(t, test.expected, string(actual))
		})
	}
}

func TestApplyYAMLNoSemanticChangePreservesBytes(t *testing.T) {
	original := []byte("image: {tag: v1} # keep compact style\n")
	actual, err := Apply(original, EditOptions{
		Method:          MethodYAMLSet,
		Content:         []byte("v1"),
		ContentProvided: true,
		Pointer:         "/image/tag",
	})
	require.NoError(t, err)
	assert.Equal(t, original, actual)
}

func TestApplyYAMLRejectsDuplicateKeys(t *testing.T) {
	_, err := Apply([]byte("image:\n  tag: v1\n  tag: v2\n"), EditOptions{
		Method:          MethodYAMLSet,
		Content:         []byte("v3"),
		ContentProvided: true,
		Pointer:         "/image/tag",
	})
	require.ErrorContains(t, err, `duplicate YAML mapping key "tag"`)
}

func TestApplyJSON(t *testing.T) {
	tests := []struct {
		name     string
		original string
		options  EditOptions
		expected string
	}{
		{
			name:     "set array value",
			original: `{"containers":[{"image":"app:v1"}]}`,
			options: EditOptions{
				Method:          MethodJSONSet,
				Content:         []byte(`"app:v2"`),
				ContentProvided: true,
				Pointer:         "/containers/0/image",
			},
			expected: "{\n  \"containers\": [\n    {\n      \"image\": \"app:v2\"\n    }\n  ]\n}\n",
		},
		{
			name:     "append array value",
			original: `{"items":["a"]}`,
			options: EditOptions{
				Method:          MethodJSONSet,
				Content:         []byte(`"b"`),
				ContentProvided: true,
				Pointer:         "/items/-",
			},
			expected: "{\n  \"items\": [\n    \"a\",\n    \"b\"\n  ]\n}\n",
		},
		{
			name:     "merge object",
			original: `{"image":{"repository":"app"},"replicas":1}`,
			options: EditOptions{
				Method:          MethodJSONMerge,
				Content:         []byte(`{"tag":"v2"}`),
				ContentProvided: true,
				Pointer:         "/image",
			},
			expected: "{\n  \"image\": {\n    \"repository\": \"app\",\n    \"tag\": \"v2\"\n  },\n  \"replicas\": 1\n}\n",
		},
		{
			name:     "RFC 6902 patch",
			original: "{\"replicas\":1}\n",
			options: EditOptions{
				Method:          MethodJSONPatch,
				Content:         []byte(`[{"op":"replace","path":"/replicas","value":2}]`),
				ContentProvided: true,
			},
			expected: "{\"replicas\":2}\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := Apply([]byte(test.original), test.options)
			require.NoError(t, err)
			assert.Equal(t, test.expected, string(actual))
		})
	}
}

func TestApplyFilePathSafety(t *testing.T) {
	worktree := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "values.yaml"), []byte("old"), 0o644))

	tests := []struct {
		name      string
		target    string
		prepare   func(*testing.T)
		errorText string
	}{
		{
			name:      "parent traversal",
			target:    "../outside",
			errorText: "escapes the repository root",
		},
		{
			name:      "git internals",
			target:    ".git/config",
			errorText: "must not be inside .git",
		},
		{
			name:   "symbolic link",
			target: "link",
			prepare: func(t *testing.T) {
				t.Helper()
				require.NoError(t, os.Symlink("values.yaml", filepath.Join(worktree, "link")))
			},
			errorText: "contains symbolic link",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.prepare != nil {
				test.prepare(t)
			}
			_, err := ApplyFile(worktree, test.target, textOptions(MethodOverwrite, "new"), false)
			require.ErrorContains(t, err, test.errorText)
		})
	}
}

func TestApplyFileCreatesEmptyFile(t *testing.T) {
	worktree := t.TempDir()
	changed, err := ApplyFile(worktree, "empty.txt", textOptions(MethodOverwrite, ""), true)
	require.NoError(t, err)
	assert.True(t, changed)
	content, err := os.ReadFile(filepath.Join(worktree, "empty.txt"))
	require.NoError(t, err)
	assert.Empty(t, content)
}

func textOptions(method Method, content string) EditOptions {
	return EditOptions{
		Method:          method,
		Content:         []byte(content),
		ContentProvided: true,
		ExpectedMatches: 1,
	}
}
