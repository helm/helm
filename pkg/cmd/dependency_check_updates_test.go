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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
)

func TestDependencyCheckUpdatesOutput(t *testing.T) {
	rows := dependencyUpdatesWriter{{Chart: "./app", Name: "db", Constraint: "^1.0.0", Locked: "1.0.0", Compatible: "1.2.0", Latest: "2.0.0"}}
	var out bytes.Buffer
	require.NoError(t, rows.WriteJSON(&out))
	var decoded []downloader.DependencyUpdate
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	assert.Equal(t, []downloader.DependencyUpdate(rows), decoded)
	out.Reset()
	require.NoError(t, rows.WriteTable(&out))
	assert.Contains(t, out.String(), "COMPATIBLE")
	assert.Contains(t, out.String(), "1.2.0")
	out.Reset()
	require.NoError(t, rows.WriteYAML(&out))
	assert.Contains(t, out.String(), "compatible: 1.2.0")
	out.Reset()
	require.NoError(t, (dependencyUpdatesWriter{}).WriteJSON(&out))
	assert.Equal(t, "[]\n", out.String())
}

func TestDependencyCheckUpdatesMultipleCharts(t *testing.T) {
	root := t.TempDir()
	oldSettings := settings
	settings = cli.New()
	settings.RegistryConfig = filepath.Join(root, "registry.json")
	settings.RepositoryConfig = filepath.Join(root, "repositories.yaml")
	settings.RepositoryCache = filepath.Join(root, "cache")
	t.Cleanup(func() { settings = oldSettings })
	paths := []string{filepath.Join(root, "frontend"), filepath.Join(root, "backend")}
	for _, path := range paths {
		require.NoError(t, os.Mkdir(path, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(path, "Chart.yaml"), []byte("apiVersion: v2\nname: app\nversion: 1.0.0\n"), 0o600))
	}
	var out, diagnostics bytes.Buffer
	cmd := newDependencyCheckUpdatesCmd(&out)
	cmd.SetErr(&diagnostics)
	cmd.SetArgs(append(paths, "-o", "json"))
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "[]\n", out.String())
	assert.Empty(t, diagnostics.String())
	cmd = newDependencyCheckUpdatesCmd(&out)
	cmd.SetArgs([]string{filepath.Join(root, "missing"), "--skip-refresh"})
	assert.ErrorContains(t, cmd.Execute(), "check updates for")
}
