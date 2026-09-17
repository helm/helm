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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitCommandIsRegistered(t *testing.T) {
	root, _, err := executeActionCommand("help git")
	require.NoError(t, err)
	command, _, err := root.Root().Find([]string{"git", "update"})
	require.NoError(t, err)
	assert.Equal(t, "update", command.Name())
	apiProvider := command.Flag("api-provider")
	require.NotNil(t, apiProvider)
	assert.Equal(t, "auto", apiProvider.DefValue)
}

func TestGitUpdateValidatesEditBeforeClone(t *testing.T) {
	_, _, err := executeActionCommand("git update https://example.invalid/repo.git --branch main --file values.yaml")
	require.ErrorContains(t, err, `method "overwrite" requires content`)
}

func TestGitUpdateRejectsMultipleContentSources(t *testing.T) {
	contentFile := filepath.Join(t.TempDir(), "content")
	require.NoError(t, os.WriteFile(contentFile, []byte("value"), 0o600))

	command := newGitUpdateCmd(os.Stdout)
	command.SetArgs([]string{
		"https://example.invalid/repo.git",
		"--branch", "main",
		"--file", "values.yaml",
		"--content", "inline",
		"--content-file", contentFile,
	})
	err := command.Execute()
	require.ErrorContains(t, err, "use only one of --content, --content-env, --content-file, or --content-stdin")
}

func TestGitUpdateRejectsContentAndContentEnvironment(t *testing.T) {
	t.Setenv("HELM_GIT_TEST_CONTENT", "from-environment")

	command := newGitUpdateCmd(os.Stdout)
	command.SetArgs([]string{
		"https://example.invalid/repo.git",
		"--branch", "main",
		"--file", "values.yaml",
		"--content", "inline",
		"--content-env", "HELM_GIT_TEST_CONTENT",
	})
	err := command.Execute()
	require.ErrorContains(t, err, "use only one of --content, --content-env, --content-file, or --content-stdin")
}

func TestGitUpdateRejectsUnsetContentEnvironment(t *testing.T) {
	command := newGitUpdateCmd(os.Stdout)
	command.SetArgs([]string{
		"https://example.invalid/repo.git",
		"--branch", "main",
		"--file", "values.yaml",
		"--content-env", "HELM_GIT_TEST_UNSET_CONTENT",
	})
	err := command.Execute()
	require.ErrorContains(t, err, `content environment variable "HELM_GIT_TEST_UNSET_CONTENT" is not set`)
}

func TestResolveCredential(t *testing.T) {
	t.Setenv("HELM_GIT_TEST_SECRET", "from-env")
	secretFile := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secretFile, []byte("from-file\r\n"), 0o600))

	tests := []struct {
		name      string
		direct    string
		filename  string
		expected  string
		errorText string
	}{
		{name: "environment", expected: "from-env"},
		{name: "direct", direct: "from-flag", expected: "from-flag"},
		{name: "file", filename: secretFile, expected: "from-file"},
		{name: "conflict", direct: "from-flag", filename: secretFile, errorText: "cannot both be set"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := resolveCredential(test.direct, test.filename, "HELM_GIT_TEST_SECRET")
			if test.errorText != "" {
				require.ErrorContains(t, err, test.errorText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}
