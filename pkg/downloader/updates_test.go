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

package downloader

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
)

func TestCheckUpdatesVersions(t *testing.T) {
	versions := []string{"invalid", "2.0.0", "1.0.0", "3.0.0-beta.1", "1.9.0", "1.2.0"}
	for _, tc := range []struct {
		constraint string
		compatible string
	}{
		{"^1.0.0", "1.9.0"},
		{"1.0.0", "1.0.0"},
		{">=3.0.0-0", "3.0.0-beta.1"},
		{">=4.0.0", ""},
	} {
		t.Run(tc.constraint, func(t *testing.T) {
			constraint, err := semver.NewConstraint(tc.constraint)
			require.NoError(t, err)
			compatible, latest := newestDependencyVersions(versions, constraint)
			assert.Equal(t, tc.compatible, compatible)
			assert.Equal(t, "2.0.0", latest)
		})
	}
}

func TestCheckUpdatesDoesNotModifyChart(t *testing.T) {
	const index = `apiVersion: v1
entries:
  child:
    - name: child
      version: 2.0.0
      urls: [child-2.0.0.tgz]
    - name: child
      version: 1.4.0
      urls: [child-1.4.0.tgz]
    - name: child
      version: 1.0.0
      urls: [child-1.0.0.tgz]
`
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		assert.Equal(t, "/index.yaml", r.URL.Path, "chart archives must not be downloaded")
		_, _ = io.WriteString(w, index)
	}))
	t.Cleanup(server.Close)
	root := t.TempDir()
	chartDir := filepath.Join(root, "parent")
	require.NoError(t, os.Mkdir(chartDir, 0o755))
	manifest := []byte("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n- name: child\n  alias: service\n  version: ^1.0.0\n  repository: '@test'\n")
	lock := []byte(fmt.Sprintf("dependencies:\n- name: child\n  version: 1.0.0\n  repository: %s\n", server.URL))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), manifest, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.lock"), lock, 0o600))
	repoFile := filepath.Join(root, "repositories.yaml")
	require.NoError(t, (&repo.File{APIVersion: "v1", Repositories: []*repo.Entry{{Name: "test", URL: server.URL}}}).WriteFile(repoFile, 0o600))
	cache := filepath.Join(root, "cache")
	require.NoError(t, os.Mkdir(cache, 0o755))
	manager := Manager{
		Out: io.Discard, ChartPath: chartDir, RepositoryConfig: repoFile, RepositoryCache: cache,
		Getters: []getter.Provider{{Schemes: []string{"http"}, New: getter.NewHTTPGetter}},
	}
	rows, err := manager.CheckUpdates()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "service", rows[0].Name)
	assert.Equal(t, "@test", rows[0].Repository)
	assert.Equal(t, "1.0.0", rows[0].Locked)
	assert.Equal(t, "1.4.0", rows[0].Compatible)
	assert.Equal(t, "2.0.0", rows[0].Latest)
	assert.Positive(t, requests.Load())
	manager.SkipUpdate = true
	requests.Store(0)
	cached, err := manager.CheckUpdates()
	require.NoError(t, err)
	assert.Equal(t, rows, cached)
	assert.Zero(t, requests.Load())
	actualManifest, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
	require.NoError(t, err)
	actualLock, err := os.ReadFile(filepath.Join(chartDir, "Chart.lock"))
	require.NoError(t, err)
	assert.Equal(t, manifest, actualManifest)
	assert.Equal(t, lock, actualLock)
	_, err = os.Stat(filepath.Join(chartDir, "charts"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.Remove(filepath.Join(cache, helmpath.CacheIndexFile("test"))))
	_, err = manager.CheckUpdates()
	assert.ErrorContains(t, err, "helm repo update")
}

func TestCheckUpdatesLocalAndEmpty(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(parent, 0o755))
	require.NoError(t, os.Mkdir(child, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(child, "Chart.yaml"), []byte("apiVersion: v2\nname: child\nversion: 1.2.0\n"), 0o600))
	manifest := []byte("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n- name: child\n  version: ^1.0.0\n  repository: file://../child\n")
	require.NoError(t, os.WriteFile(filepath.Join(parent, "Chart.yaml"), manifest, 0o600))
	manager := Manager{Out: io.Discard, ChartPath: parent, RepositoryConfig: filepath.Join(root, "missing.yaml")}
	rows, err := manager.CheckUpdates()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "1.2.0", rows[0].Compatible)
	assert.Empty(t, rows[0].Locked)
	require.NoError(t, os.WriteFile(filepath.Join(parent, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 1.0.0\n"), 0o600))
	rows, err = manager.CheckUpdates()
	require.NoError(t, err)
	assert.Equal(t, []DependencyUpdate{}, rows)
}

func TestCheckUpdatesLockMatching(t *testing.T) {
	dependency := &chart.Dependency{Name: "child", Repository: "https://charts.example"}
	lock := &chart.Lock{Dependencies: []*chart.Dependency{
		{Name: "child", Repository: "https://charts.example", Version: "1.0.0"},
		{Name: "child", Repository: "https://charts.example", Version: "2.0.0"},
	}}
	assert.Equal(t, "2.0.0", lockedDependencyVersion(lock, dependency, "@repo", 1))
	assert.Empty(t, lockedDependencyVersion(lock, dependency, "@repo", 2))
	assert.Empty(t, lockedDependencyVersion(nil, dependency, "@repo", 0))
}

func TestCheckUpdatesOCI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/":
			_, _ = io.WriteString(w, `{}`)
		case "/v2/charts/child/tags/list":
			_, _ = io.WriteString(w, `{"name":"charts/child","tags":["2.0.0","1.1.0","1.0.0"]}`)
		default:
			t.Errorf("unexpected registry request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	root := t.TempDir()
	repository := "oci://" + strings.TrimPrefix(server.URL, "http://") + "/charts"
	manifest := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n- name: child\n  version: ^1.0.0\n  repository: %s\n", repository)
	require.NoError(t, os.WriteFile(filepath.Join(root, "Chart.yaml"), []byte(manifest), 0o600))
	client, err := registry.NewClient(registry.ClientOptPlainHTTP(), registry.ClientOptCredentialsFile(filepath.Join(root, "credentials.json")))
	require.NoError(t, err)
	manager := Manager{Out: io.Discard, ChartPath: root, RegistryClient: client, SkipUpdate: true}
	rows, err := manager.CheckUpdates()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "1.1.0", rows[0].Compatible)
	assert.Equal(t, "2.0.0", rows[0].Latest)
	manager.RegistryClient = nil
	_, err = manager.CheckUpdates()
	assert.ErrorContains(t, err, "OCI registry client is required")
}
