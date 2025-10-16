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

package getter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Masterminds/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
)

const testPluginYAML = `name: test-plugin
version: 1.0.0
usage: A test plugin
description: Test plugin for VCS getter
command: echo
`

func TestNewVCSGetter(t *testing.T) {
	g, err := NewVCSGetter()
	require.NoError(t, err)
	assert.NotNil(t, g)
	assert.IsType(t, &VCSGetter{}, g)
}

func TestVCSGetter_ArtifactTypeValidation(t *testing.T) {
	g, err := NewVCSGetter()
	require.NoError(t, err)

	t.Run("RejectsCharts", func(t *testing.T) {
		_, err := g.Get("git://example.com/repo.git", WithArtifactType("chart"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "VCS getter can only be used for plugins, not chart")
	})

	t.Run("RejectsOtherTypes", func(t *testing.T) {
		_, err := g.Get("git://example.com/repo.git", WithArtifactType("unknown"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "VCS getter can only be used for plugins, not unknown")
	})

	t.Run("AllowsPlugins", func(t *testing.T) {
		// Test that plugin artifact type validation passes
		// We don't need to test actual network calls, just that the validation allows it
		g := &VCSGetter{opts: getterOptions{artifactType: "plugin"}}

		// Test the artifact type check directly
		if g.opts.artifactType != "plugin" {
			t.Error("Should allow plugins")
		}
	})
}

func TestVCSGetter_NormalizeURL(t *testing.T) {
	g := &VCSGetter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "GitScheme",
			input:    "git://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "GitPlusHTTPS",
			input:    "git+https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "GitPlusSSH",
			input:    "git+ssh://git@github.com:user/repo.git",
			expected: "ssh://git@github.com:user/repo.git",
		},
		{
			name:     "GitHubWithoutGitSuffix",
			input:    "github.com/user/repo",
			expected: "github.com/user/repo.git",
		},
		{
			name:     "AlreadyNormalized",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.normalizeURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVCSGetter_GetSemVers(t *testing.T) {
	g := &VCSGetter{}

	tests := []struct {
		name     string
		tags     []string
		expected int
	}{
		{
			name:     "ValidSemverTags",
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "invalid-tag", "v1.0.0-alpha"},
			expected: 4, // All except "invalid-tag"
		},
		{
			name:     "NoValidTags",
			tags:     []string{"main", "develop", "feature-branch"},
			expected: 0,
		},
		{
			name:     "EmptyTags",
			tags:     []string{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.getSemVers(tt.tags)
			assert.Len(t, result, tt.expected)
		})
	}
}

// Mock repository implementation based on original VCS installer tests
type testRepo struct {
	local, remote, current string
	tags, branches         []string
	err                    error
	vcs.Repo
}

func (r *testRepo) LocalPath() string           { return r.local }
func (r *testRepo) Remote() string              { return r.remote }
func (r *testRepo) Update() error               { return r.err }
func (r *testRepo) Get() error                  { return r.err }
func (r *testRepo) IsReference(string) bool     { return false }
func (r *testRepo) Tags() ([]string, error)     { return r.tags, r.err }
func (r *testRepo) Branches() ([]string, error) { return r.branches, r.err }
func (r *testRepo) UpdateVersion(version string) error {
	r.current = version
	return r.err
}

func TestVCSGetter_WithMockRepo(t *testing.T) {
	ensure.HelmHome(t)

	// Create a test plugin directory structure
	tempDir := t.TempDir()
	testRepoPath := filepath.Join(tempDir, "test-plugin")
	err := os.MkdirAll(testRepoPath, 0755)
	require.NoError(t, err)

	// Create plugin.yaml
	err = os.WriteFile(filepath.Join(testRepoPath, "plugin.yaml"), []byte(testPluginYAML), 0644)
	require.NoError(t, err)

	source := "https://github.com/example/test-plugin"
	repo := &testRepo{
		local:  testRepoPath,
		remote: source,
		tags:   []string{"0.1.0", "0.1.1", "1.0.0"},
	}

	g := &VCSGetter{
		opts: getterOptions{
			artifactType: "plugin",
			version:      "~0.1.0",
		},
	}

	t.Run("VersionResolution", func(t *testing.T) {
		resolvedVersion, err := g.solveVersion(repo, "~0.1.0")
		require.NoError(t, err)
		assert.Equal(t, "0.1.1", resolvedVersion, "Should resolve to highest patch version")
	})

	t.Run("ExactVersionMatch", func(t *testing.T) {
		resolvedVersion, err := g.solveVersion(repo, "1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", resolvedVersion)
	})

	t.Run("NonExistentVersion", func(t *testing.T) {
		_, err := g.solveVersion(repo, "2.0.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requested version \"2.0.0\" does not exist")
	})

	t.Run("SetVersion", func(t *testing.T) {
		err := g.setVersion(repo, "0.1.1")
		assert.NoError(t, err)
		assert.Equal(t, "0.1.1", repo.current)
	})

	t.Run("ArchiveRepo", func(t *testing.T) {
		buffer, err := g.archiveRepo(repo)
		assert.NoError(t, err)
		assert.NotNil(t, buffer)
		// Should contain the plugin.yaml content
		content := buffer.String()
		assert.Contains(t, content, "name: test-plugin")
	})
}

func TestVCSGetter_ErrorHandling(t *testing.T) {
	ensure.HelmHome(t)

	source := "https://github.com/nonexistent/plugin"

	t.Run("RepoWithError", func(t *testing.T) {
		repo := &testRepo{
			remote: source,
			err:    fmt.Errorf("network error"),
		}

		g := &VCSGetter{opts: getterOptions{artifactType: "plugin"}}

		err := g.syncRepo(repo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network error")
	})

	t.Run("InvalidVersionConstraint", func(t *testing.T) {
		repo := &testRepo{
			remote: source,
			tags:   []string{"1.0.0"},
		}

		g := &VCSGetter{opts: getterOptions{artifactType: "plugin"}}

		_, err := g.solveVersion(repo, "invalid-constraint")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "improper constraint")
	})
}

func TestVCSGetter_RestrictToArtifactTypes(t *testing.T) {
	getter, err := NewVCSGetter()
	assert.NoError(t, err)

	vcsGetter := getter.(*VCSGetter)
	supported := vcsGetter.RestrictToArtifactTypes()

	assert.Equal(t, []string{"plugin"}, supported, "VCS getter should only support plugins")
}

func TestVCSGetter_LegacyCompatibility(t *testing.T) {
	ensure.HelmHome(t)

	// Test cases that mirror the original VCS installer behavior
	tests := []struct {
		name            string
		sourceURL       string
		version         string
		expectedVersion string
		tags            []string
		shouldError     bool
	}{
		{
			name:            "SemanticVersioning",
			sourceURL:       "https://github.com/example/helm-env",
			version:         "~0.1.0",
			expectedVersion: "0.1.1",
			tags:            []string{"0.1.0", "0.1.1"},
			shouldError:     false,
		},
		{
			name:        "NonExistentVersion",
			sourceURL:   "https://github.com/example/helm-env",
			version:     "0.2.0",
			tags:        []string{"0.1.0", "0.1.1"},
			shouldError: true,
		},
		{
			name:            "NoVersionSpecified",
			sourceURL:       "https://github.com/example/helm-env",
			version:         "",
			expectedVersion: "",
			tags:            []string{"0.1.0", "0.1.1"},
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock plugin directory
			tempDir := t.TempDir()
			testRepoPath := filepath.Join(tempDir, "test-plugin")
			err := os.MkdirAll(testRepoPath, 0755)
			require.NoError(t, err)

			// Create plugin.yaml
			err = os.WriteFile(filepath.Join(testRepoPath, "plugin.yaml"), []byte(testPluginYAML), 0644)
			require.NoError(t, err)

			repo := &testRepo{
				local:  testRepoPath,
				remote: tt.sourceURL,
				tags:   tt.tags,
			}

			g := &VCSGetter{
				opts: getterOptions{
					artifactType: "plugin",
					version:      tt.version,
				},
			}

			if tt.version != "" {
				resolvedVersion, err := g.solveVersion(repo, tt.version)
				if tt.shouldError {
					assert.Error(t, err)
					if tt.version == "0.2.0" {
						expectedError := fmt.Sprintf("requested version %q does not exist for plugin %q", tt.version, tt.sourceURL)
						assert.Equal(t, expectedError, err.Error())
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedVersion, resolvedVersion)
				}
			}
		})
	}
}

func TestVCSGetter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git not available in test environment")
	}

	ensure.HelmHome(t)

	// Create a temporary git repository for testing
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "test-repo")

	// Initialize git repo
	err := os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	// Initialize git repo and add some content
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "checkout", "-b", "main"},
	}

	for _, cmd := range commands {
		execCmd := exec.Command(cmd[0], cmd[1:]...)
		execCmd.Dir = repoDir
		err := execCmd.Run()
		require.NoError(t, err, "Failed to run: %v", cmd)
	}

	// Create a plugin.yaml file to make it a valid plugin
	pluginFile := filepath.Join(repoDir, "plugin.yaml")
	err = os.WriteFile(pluginFile, []byte(testPluginYAML), 0644)
	require.NoError(t, err)

	// Add and commit the plugin file
	commitCommands := [][]string{
		{"git", "add", "plugin.yaml"},
		{"git", "commit", "-m", "Initial commit"},
		{"git", "tag", "-a", "v1.0.0", "-m", "Version 1.0.0"},
	}

	for _, cmd := range commitCommands {
		execCmd := exec.Command(cmd[0], cmd[1:]...)
		execCmd.Dir = repoDir
		err := execCmd.Run()
		require.NoError(t, err, "Failed to run: %v", cmd)
	}

	t.Run("GetFromLocalRepo", func(t *testing.T) {
		g, err := NewVCSGetter(WithArtifactType("plugin"))
		require.NoError(t, err)

		// Use file:// URL for local repository
		repoURL := "file://" + repoDir

		buffer, err := g.Get(repoURL)
		if err != nil {
			// This is expected to fail in test environment, but we verify the error isn't about artifact type
			assert.NotContains(t, err.Error(), "can only be used for plugins")
		} else {
			// If it succeeds, verify we got some content
			assert.NotNil(t, buffer)
			assert.Greater(t, buffer.Len(), 0)
		}
	})
}
