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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo/repotest"
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

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name, base, path, expect string
	}{
		{name: "basic URL", base: "https://example.com", path: "http://helm.sh/foo", expect: "http://helm.sh/foo"},
		{name: "relative path", base: "https://helm.sh/charts", path: "foo", expect: "https://helm.sh/charts/foo"},
		{name: "Encoded path", base: "https://helm.sh/a%2Fb/charts", path: "foo", expect: "https://helm.sh/a%2Fb/charts/foo"},
	}

	for _, tt := range tests {
		got, err := normalizeURL(tt.base, tt.path)
		if err != nil {
			t.Errorf("%s: error %s", tt.name, err)
			continue
		} else if got != tt.expect {
			t.Errorf("%s: expected %q, got %q", tt.name, tt.expect, got)
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
	chartPath, err := ioutil.TempDir("", "test-downloadall")
	if err != nil {
		t.Fatalf("could not create tempdir: %v", err)
	}
	defer os.RemoveAll(chartPath)
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

	if _, err := os.Stat(filepath.Join(chartPath, "charts", "signtest-0.1.0.tgz")); os.IsNotExist(err) {
		t.Error(err)
	}
}

func TestUpdateBeforeBuild(t *testing.T) {
	// Set up a fake repo
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
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
	err = m.Update()
	if err != nil {
		t.Fatal(err)
	}

	err = m.Build()
	if err != nil {
		t.Fatal(err)
	}
}

// TestUpdateWithNoRepo is for the case of a dependency that has no repo listed.
// This happens when the dependency is in the charts directory and does not need
// to be fetched.
func TestUpdateWithNoRepo(t *testing.T) {
	// Set up a fake repo
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
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
	err = m.Update()
	if err != nil {
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
	// Set up a fake repo
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
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
	m := &Manager{
		ChartPath:        dir(chartName),
		Out:              b,
		Getters:          g,
		RepositoryConfig: dir("repositories.yaml"),
		RepositoryCache:  dir(),
	}

	// First build will update dependencies and create Chart.lock file.
	err = m.Build()
	if err != nil {
		t.Fatal(err)
	}

	// Second build should be passed. See PR #6655.
	err = m.Build()
	if err != nil {
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
