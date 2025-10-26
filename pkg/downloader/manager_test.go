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
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestVersionEquals(t *testing.T) {
	tests := []struct {
		name, v1, v2 string
		expect       bool
	}{
		{name: "semver match", v1: "1.2.3-beta.11", v2: "1.2.3-beta.11", expect: true},
		{name: "semver match, build info", v1: "1.2.3-beta.11+a", v2: "1.2.3-beta.11+b", expect: true},
		{name: "string match", v1: "abcdef123", v2: "abcdef123", expect: true},
		{name: "semver mismatch", v1: "1.2.3-beta.11", v2: "1.2.3-beta.22", expect: false},
		{name: "semver mismatch, invalid semver", v1: "1.2.3-beta.11", v2: "stinkycheese", expect: false},
	}

	for _, tt := range tests {
		if versionEquals(tt.v1, tt.v2) != tt.expect {
			t.Errorf("%s: failed comparison of %q and %q (expect equal: %t)", tt.name, tt.v1, tt.v2, tt.expect)
		}
	}
}

func TestFindChartURL(t *testing.T) {
	var b bytes.Buffer
	m := &Manager{
		Out:              &b,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
	}
	repos, err := m.loadChartRepositories()
	if err != nil {
		t.Fatal(err)
	}

	name := "alpine"
	version := "0.1.0"
	repoURL := "http://example.com/charts"

	churl, username, password, insecureSkipTLSVerify, passcredentialsall, _, _, _, err := m.findChartURL(name, version, repoURL, repos)
	if err != nil {
		t.Fatal(err)
	}

	if churl != "https://charts.helm.sh/stable/alpine-0.1.0.tgz" {
		t.Errorf("Unexpected URL %q", churl)
	}
	if username != "" {
		t.Errorf("Unexpected username %q", username)
	}
	if password != "" {
		t.Errorf("Unexpected password %q", password)
	}
	if passcredentialsall != false {
		t.Errorf("Unexpected passcredentialsall %t", passcredentialsall)
	}
	if insecureSkipTLSVerify {
		t.Errorf("Unexpected insecureSkipTLSVerify %t", insecureSkipTLSVerify)
	}

	name = "tlsfoo"
	version = "1.2.3"
	repoURL = "https://example-https-insecureskiptlsverify.com"

	churl, username, password, insecureSkipTLSVerify, passcredentialsall, _, _, _, err = m.findChartURL(name, version, repoURL, repos)
	if err != nil {
		t.Fatal(err)
	}

	if !insecureSkipTLSVerify {
		t.Errorf("Unexpected insecureSkipTLSVerify %t", insecureSkipTLSVerify)
	}
	if churl != "https://example.com/tlsfoo-1.2.3.tgz" {
		t.Errorf("Unexpected URL %q", churl)
	}
	if username != "" {
		t.Errorf("Unexpected username %q", username)
	}
	if password != "" {
		t.Errorf("Unexpected password %q", password)
	}
	if passcredentialsall != false {
		t.Errorf("Unexpected passcredentialsall %t", passcredentialsall)
	}

	name = "foo"
	version = "1.2.3"
	repoURL = "http://example.com/helm"

	churl, username, password, insecureSkipTLSVerify, passcredentialsall, _, _, _, err = m.findChartURL(name, version, repoURL, repos)
	if err != nil {
		t.Fatal(err)
	}

	if churl != "http://example.com/helm/charts/foo-1.2.3.tgz" {
		t.Errorf("Unexpected URL %q", churl)
	}
	if username != "" {
		t.Errorf("Unexpected username %q", username)
	}
	if password != "" {
		t.Errorf("Unexpected password %q", password)
	}
	if passcredentialsall != false {
		t.Errorf("Unexpected passcredentialsall %t", passcredentialsall)
	}
	if insecureSkipTLSVerify {
		t.Errorf("Unexpected insecureSkipTLSVerify %t", insecureSkipTLSVerify)
	}
}

func TestGetRepoNames(t *testing.T) {
	b := bytes.NewBuffer(nil)
	m := &Manager{
		Out:              b,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
	}
	tests := []struct {
		name   string
		req    []*chart.Dependency
		expect map[string]string
		err    bool
	}{
		{
			name: "no repo definition, but references a url",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "http://example.com/test"},
			},
			expect: map[string]string{"http://example.com/test": "http://example.com/test"},
		},
		{
			name: "no repo definition failure -- stable repo",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "stable"},
			},
			err: true,
		},
		{
			name: "no repo definition failure",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "http://example.com"},
			},
			expect: map[string]string{"oedipus-rex": "testing"},
		},
		{
			name: "repo from local path",
			req: []*chart.Dependency{
				{Name: "local-dep", Repository: "file://./testdata/signtest"},
			},
			expect: map[string]string{"local-dep": "file://./testdata/signtest"},
		},
		{
			name: "repo alias (alias:)",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "alias:testing"},
			},
			expect: map[string]string{"oedipus-rex": "testing"},
		},
		{
			name: "repo alias (@)",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "@testing"},
			},
			expect: map[string]string{"oedipus-rex": "testing"},
		},
		{
			name: "repo from local chart under charts path",
			req: []*chart.Dependency{
				{Name: "local-subchart", Repository: ""},
			},
			expect: map[string]string{},
		},
	}

	for _, tt := range tests {
		l, err := m.resolveRepoNames(tt.req)
		if err != nil {
			if tt.err {
				continue
			}
			t.Fatal(err)
		}

		if tt.err {
			t.Fatalf("Expected error in test %q", tt.name)
		}

		// m1 and m2 are the maps we want to compare
		eq := reflect.DeepEqual(l, tt.expect)
		if !eq {
			t.Errorf("%s: expected map %v, got %v", tt.name, l, tt.name)
		}
	}
}

func TestDownloadAll(t *testing.T) {
	chartPath := t.TempDir()
	m := &Manager{
		Out:              new(bytes.Buffer),
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		ChartPath:        chartPath,
	}
	signtest, err := loader.LoadDir(filepath.Join("testdata", "signtest"))
	if err != nil {
		t.Fatal(err)
	}
	if err := chartutil.SaveDir(signtest, filepath.Join(chartPath, "testdata")); err != nil {
		t.Fatal(err)
	}

	local, err := loader.LoadDir(filepath.Join("testdata", "local-subchart"))
	if err != nil {
		t.Fatal(err)
	}
	if err := chartutil.SaveDir(local, filepath.Join(chartPath, "charts")); err != nil {
		t.Fatal(err)
	}

	signDep := &chart.Dependency{
		Name:       signtest.Name(),
		Repository: "file://./testdata/signtest",
		Version:    signtest.Metadata.Version,
	}
	localDep := &chart.Dependency{
		Name:       local.Name(),
		Repository: "",
		Version:    local.Metadata.Version,
	}

	// create a 'tmpcharts' directory to test #5567
	if err := os.MkdirAll(filepath.Join(chartPath, "tmpcharts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := m.downloadAll([]*chart.Dependency{signDep, localDep}); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(filepath.Join(chartPath, "charts", "signtest-0.1.0.tgz")); errors.Is(err, fs.ErrNotExist) {
		t.Error(err)
	}

	// A chart with a bad name like this cannot be loaded and saved. Handling in
	// the loading and saving will return an error about the invalid name. In
	// this case, the chart needs to be created directly.
	badchartyaml := `apiVersion: v2
description: A Helm chart for Kubernetes
name: ../bad-local-subchart
version: 0.1.0`
	if err := os.MkdirAll(filepath.Join(chartPath, "testdata", "bad-local-subchart"), 0755); err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(chartPath, "testdata", "bad-local-subchart", "Chart.yaml"), []byte(badchartyaml), 0644)
	if err != nil {
		t.Fatal(err)
	}

	badLocalDep := &chart.Dependency{
		Name:       "../bad-local-subchart",
		Repository: "file://./testdata/bad-local-subchart",
		Version:    "0.1.0",
	}

	err = m.downloadAll([]*chart.Dependency{badLocalDep})
	if err == nil {
		t.Fatal("Expected error for bad dependency name")
	}
}

func TestUpdateBeforeBuild(t *testing.T) {
	// Set up a fake repo
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
	)
	defer srv.Stop()
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}
	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}

	// Save dep
	d := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "dep-chart",
			Version:    "0.1.0",
			APIVersion: "v1",
		},
	}
	if err := chartutil.SaveDir(d, dir()); err != nil {
		t.Fatal(err)
	}
	// Save a chart
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "with-dependency",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{{
				Name:       d.Metadata.Name,
				Version:    ">=0.1.0",
				Repository: "file://../dep-chart",
			}},
		},
	}
	if err := chartutil.SaveDir(c, dir()); err != nil {
		t.Fatal(err)
	}

	// Set-up a manager
	b := bytes.NewBuffer(nil)
	g := getter.Providers{getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	}}
	m := &Manager{
		ChartPath:        dir(c.Metadata.Name),
		Out:              b,
		Getters:          g,
		RepositoryConfig: dir("repositories.yaml"),
		RepositoryCache:  dir(),
	}

	// Update before Build. see issue: https://github.com/helm/helm/issues/7101
	if err := m.Update(); err != nil {
		t.Fatal(err)
	}

	if err := m.Build(); err != nil {
		t.Fatal(err)
	}
}

// TestUpdateWithNoRepo is for the case of a dependency that has no repo listed.
// This happens when the dependency is in the charts directory and does not need
// to be fetched.
func TestUpdateWithNoRepo(t *testing.T) {
	// Set up a fake repo
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
	)
	defer srv.Stop()
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}
	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}

	// Setup the dependent chart
	d := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "dep-chart",
			Version:    "0.1.0",
			APIVersion: "v1",
		},
	}

	// Save a chart with the dependency
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "with-dependency",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{{
				Name:    d.Metadata.Name,
				Version: "0.1.0",
			}},
		},
	}
	if err := chartutil.SaveDir(c, dir()); err != nil {
		t.Fatal(err)
	}

	// Save dependent chart into the parents charts directory. If the chart is
	// not in the charts directory Helm will return an error that it is not
	// found.
	if err := chartutil.SaveDir(d, dir(c.Metadata.Name, "charts")); err != nil {
		t.Fatal(err)
	}

	// Set-up a manager
	b := bytes.NewBuffer(nil)
	g := getter.Providers{getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	}}
	m := &Manager{
		ChartPath:        dir(c.Metadata.Name),
		Out:              b,
		Getters:          g,
		RepositoryConfig: dir("repositories.yaml"),
		RepositoryCache:  dir(),
	}

	// Test the update
	if err := m.Update(); err != nil {
		t.Fatal(err)
	}
}

// This function is the skeleton test code of failing tests for #6416 and #6871 and bugs due to #5874.
//
// This function is used by below tests that ensures success of build operation
// with optional fields, alias, condition, tags, and even with ranged version.
// Parent chart includes local-subchart 0.1.0 subchart from a fake repository, by default.
// If each of these main fields (name, version, repository) is not supplied by dep param, default value will be used.
func checkBuildWithOptionalFields(t *testing.T, chartName string, dep chart.Dependency) {
	t.Helper()
	// Set up a fake repo
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/*.tgz*"),
	)
	defer srv.Stop()
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}
	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}

	// Set main fields if not exist
	if dep.Name == "" {
		dep.Name = "local-subchart"
	}
	if dep.Version == "" {
		dep.Version = "0.1.0"
	}
	if dep.Repository == "" {
		dep.Repository = srv.URL()
	}

	// Save a chart
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         chartName,
			Version:      "0.1.0",
			APIVersion:   "v2",
			Dependencies: []*chart.Dependency{&dep},
		},
	}
	if err := chartutil.SaveDir(c, dir()); err != nil {
		t.Fatal(err)
	}

	// Set-up a manager
	b := bytes.NewBuffer(nil)
	g := getter.Providers{getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	}}
	contentCache := t.TempDir()
	m := &Manager{
		ChartPath:        dir(chartName),
		Out:              b,
		Getters:          g,
		RepositoryConfig: dir("repositories.yaml"),
		RepositoryCache:  dir(),
		ContentCache:     contentCache,
	}

	// First build will update dependencies and create Chart.lock file.
	if err := m.Build(); err != nil {
		t.Fatal(err)
	}

	// Second build should be passed. See PR #6655.
	if err := m.Build(); err != nil {
		t.Fatal(err)
	}
}

func TestBuild_WithoutOptionalFields(t *testing.T) {
	// Dependency has main fields only (name/version/repository)
	checkBuildWithOptionalFields(t, "without-optional-fields", chart.Dependency{})
}

func TestBuild_WithSemVerRange(t *testing.T) {
	// Dependency version is the form of SemVer range
	checkBuildWithOptionalFields(t, "with-semver-range", chart.Dependency{
		Version: ">=0.1.0",
	})
}

func TestBuild_WithAlias(t *testing.T) {
	// Dependency has an alias
	checkBuildWithOptionalFields(t, "with-alias", chart.Dependency{
		Alias: "local-subchart-alias",
	})
}

func TestBuild_WithCondition(t *testing.T) {
	// Dependency has a condition
	checkBuildWithOptionalFields(t, "with-condition", chart.Dependency{
		Condition: "some.condition",
	})
}

func TestBuild_WithTags(t *testing.T) {
	// Dependency has several tags
	checkBuildWithOptionalFields(t, "with-tags", chart.Dependency{
		Tags: []string{"tag1", "tag2"},
	})
}

// Failing test for #6871
func TestBuild_WithRepositoryAlias(t *testing.T) {
	// Dependency repository is aliased in Chart.yaml
	checkBuildWithOptionalFields(t, "with-repository-alias", chart.Dependency{
		Repository: "@test",
	})
}

func TestErrRepoNotFound_Error(t *testing.T) {
	type fields struct {
		Repos []string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "OK",
			fields: fields{
				Repos: []string{"https://charts1.example.com", "https://charts2.example.com"},
			},
			want: "no repository definition for https://charts1.example.com, https://charts2.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ErrRepoNotFound{
				Repos: tt.fields.Repos,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKey(t *testing.T) {
	tests := []struct {
		name   string
		expect string
	}{
		{
			name:   "file:////tmp",
			expect: "afeed3459e92a874f6373aca264ce1459bfa91f9c1d6612f10ae3dc2ee955df3",
		},
		{
			name:   "https://example.com/charts",
			expect: "7065c57c94b2411ad774638d76823c7ccb56415441f5ab2f5ece2f3845728e5d",
		},
		{
			name:   "foo/bar/baz",
			expect: "15c46a4f8a189ae22f36f201048881d6c090c93583bedcf71f5443fdef224c82",
		},
	}

	for _, tt := range tests {
		o, err := key(tt.name)
		if err != nil {
			t.Fatalf("unable to generate key for %q with error: %s", tt.name, err)
		}
		if o != tt.expect {
			t.Errorf("wrong key name generated for %q, expected %q but got %q", tt.name, tt.expect, o)
		}
	}
}

// Test dedupeRepos tests that the dedupeRepos function correctly deduplicates
func TestDedupeRepos(t *testing.T) {
	tests := []struct {
		name  string
		repos []*repo.Entry
		want  []*repo.Entry
	}{
		{
			name: "no duplicates",
			repos: []*repo.Entry{
				{
					URL: "https://example.com/charts",
				},
				{
					URL: "https://example.com/charts2",
				},
			},
			want: []*repo.Entry{
				{
					URL: "https://example.com/charts",
				},
				{
					URL: "https://example.com/charts2",
				},
			},
		},
		{
			name: "duplicates",
			repos: []*repo.Entry{
				{
					URL: "https://example.com/charts",
				},
				{
					URL: "https://example.com/charts",
				},
			},
			want: []*repo.Entry{
				{
					URL: "https://example.com/charts",
				},
			},
		},
		{
			name: "duplicates with trailing slash",
			repos: []*repo.Entry{
				{
					URL: "https://example.com/charts",
				},
				{
					URL: "https://example.com/charts/",
				},
			},
			want: []*repo.Entry{
				{
					// the last one wins
					URL: "https://example.com/charts/",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupeRepos(tt.repos)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestWriteLock(t *testing.T) {
	fixedTime, err := time.Parse(time.RFC3339, "2025-07-04T00:00:00Z")
	assert.NoError(t, err)
	lock := &chart.Lock{
		Generated: fixedTime,
		Digest:    "sha256:12345",
		Dependencies: []*chart.Dependency{
			{
				Name:       "fantastic-chart",
				Version:    "1.2.3",
				Repository: "https://example.com/charts",
			},
		},
	}
	expectedContent, err := yaml.Marshal(lock)
	assert.NoError(t, err)

	t.Run("v2 lock file", func(t *testing.T) {
		dir := t.TempDir()
		err := writeLock(dir, lock, false)
		assert.NoError(t, err)

		lockfilePath := filepath.Join(dir, "Chart.lock")
		_, err = os.Stat(lockfilePath)
		assert.NoError(t, err, "Chart.lock should exist")

		content, err := os.ReadFile(lockfilePath)
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, content)

		// Check that requirements.lock does not exist
		_, err = os.Stat(filepath.Join(dir, "requirements.lock"))
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("v1 lock file", func(t *testing.T) {
		dir := t.TempDir()
		err := writeLock(dir, lock, true)
		assert.NoError(t, err)

		lockfilePath := filepath.Join(dir, "requirements.lock")
		_, err = os.Stat(lockfilePath)
		assert.NoError(t, err, "requirements.lock should exist")

		content, err := os.ReadFile(lockfilePath)
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, content)

		// Check that Chart.lock does not exist
		_, err = os.Stat(filepath.Join(dir, "Chart.lock"))
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("overwrite existing lock file", func(t *testing.T) {
		dir := t.TempDir()
		lockfilePath := filepath.Join(dir, "Chart.lock")
		assert.NoError(t, os.WriteFile(lockfilePath, []byte("old content"), 0644))

		err = writeLock(dir, lock, false)
		assert.NoError(t, err)

		content, err := os.ReadFile(lockfilePath)
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, content)
	})

	t.Run("lock file is a symlink", func(t *testing.T) {
		dir := t.TempDir()
		dummyFile := filepath.Join(dir, "dummy.txt")
		assert.NoError(t, os.WriteFile(dummyFile, []byte("dummy"), 0644))

		lockfilePath := filepath.Join(dir, "Chart.lock")
		assert.NoError(t, os.Symlink(dummyFile, lockfilePath))

		err = writeLock(dir, lock, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "the Chart.lock file is a symlink to")
	})

	t.Run("chart path is not a directory", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "not-a-dir")
		assert.NoError(t, os.WriteFile(filePath, []byte("file"), 0644))

		err = writeLock(filePath, lock, false)
		assert.Error(t, err)
	})
}
