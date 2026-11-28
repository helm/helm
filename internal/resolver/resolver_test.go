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

package resolver

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/registry"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		req    []*chart.Dependency
		expect *chart.Lock
		err    bool
	}{
		{
			name: "repo from invalid version",
			req: []*chart.Dependency{
				{Name: "base", Repository: "file://base", Version: "1.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "base", Repository: "file://base", Version: "0.1.0"},
				},
			},
			err: true,
		},
		{
			name: "version failure",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "http://example.com", Version: ">a1"},
			},
			err: true,
		},
		{
			name: "cache index failure",
			req: []*chart.Dependency{
				{Name: "oedipus-rex", Repository: "http://example.com", Version: "1.0.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "oedipus-rex", Repository: "http://example.com", Version: "1.0.0"},
				},
			},
		},
		{
			name: "chart not found failure",
			req: []*chart.Dependency{
				{Name: "redis", Repository: "http://example.com", Version: "1.0.0"},
			},
			err: true,
		},
		{
			name: "constraint not satisfied failure",
			req: []*chart.Dependency{
				{Name: "alpine", Repository: "http://example.com", Version: ">=1.0.0"},
			},
			err: true,
		},
		{
			name: "valid lock",
			req: []*chart.Dependency{
				{Name: "alpine", Repository: "http://example.com", Version: ">=0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "alpine", Repository: "http://example.com", Version: "0.2.0"},
				},
			},
		},
		{
			name: "repo from valid local path",
			req: []*chart.Dependency{
				{Name: "base", Repository: "file://base", Version: "0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "base", Repository: "file://base", Version: "0.1.0"},
				},
			},
		},
		{
			name: "repo from valid local path with range resolution",
			req: []*chart.Dependency{
				{Name: "base", Repository: "file://base", Version: "^0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "base", Repository: "file://base", Version: "0.1.0"},
				},
			},
		},
		{
			name: "repo from invalid local path",
			req: []*chart.Dependency{
				{Name: "nonexistent", Repository: "file://testdata/nonexistent", Version: "0.1.0"},
			},
			err: true,
		},
		{
			name: "repo from valid path under charts path",
			req: []*chart.Dependency{
				{Name: "localdependency", Repository: "", Version: "0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "localdependency", Repository: "", Version: "0.1.0"},
				},
			},
		},
		{
			name: "repo from invalid path under charts path",
			req: []*chart.Dependency{
				{Name: "nonexistentdependency", Repository: "", Version: "0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "nonexistentlocaldependency", Repository: "", Version: "0.1.0"},
				},
			},
			err: true,
		},
	}

	repoNames := map[string]string{"alpine": "kubernetes-charts", "redis": "kubernetes-charts"}
	registryClient, _ := registry.NewClient()
	r := New("testdata/chartpath", "testdata/repository", registryClient)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := r.Resolve(tt.req, repoNames)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				h, err := HashReq(tt.req, tt.expect.Dependencies)
				require.NoError(t, err)
				assert.Equal(t, l.Digest, h, "%q: hashes don't match.", tt.name)

				// Check fields.
				require.Len(t, l.Dependencies, len(tt.req), "%s: wrong number of dependencies in lock", tt.name)
				d0 := l.Dependencies[0]
				e0 := tt.expect.Dependencies[0]
				assert.Equal(t, e0.Name, d0.Name, tt.name)
				assert.Equal(t, e0.Repository, d0.Repository, tt.name)
				assert.Equal(t, e0.Version, d0.Version, tt.name)
			}
		})
	}
}

// TestResolveOCIUnderscoreTag reproduces the dependency resolution failure
// from #32582: a chart stored in an OCI registry under the underscore form
// of a build-metadata tag (1.1.17_meta, per #10166) must resolve against a
// version constraint during helm dep update.
func TestResolveOCIUnderscoreTag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/testrepo/my-subchart/tags/list", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"testrepo/my-subchart","tags":["1.1.17_meta","0.9.0"]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	registryClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)

	// The reported failure is in what Tags hands to the resolver and to
	// pull references: without the fix it returns 1.1.17+meta, a tag that
	// does not exist in the registry, while the resolution below happens
	// to succeed on the normalized form either way. Assert the tags are
	// the stored strings so this test fails without the fix.
	tags, err := registryClient.Tags(strings.TrimPrefix(server.URL, "http://") + "/testrepo/my-subchart")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.1.17_meta", "0.9.0"}, tags)

	r := New("", "", registryClient)

	repoURL := "oci://" + strings.TrimPrefix(server.URL, "http://") + "/testrepo"
	req := []*chart.Dependency{
		{Name: "my-subchart", Repository: repoURL, Version: "1.x.x"},
	}
	l, err := r.Resolve(req, map[string]string{"my-subchart": "testrepo"})
	require.NoError(t, err)
	require.Len(t, l.Dependencies, 1)
	assert.Equal(t, "1.1.17+meta", l.Dependencies[0].Version)
}

func TestHashReq(t *testing.T) {
	expect := "sha256:fb239e836325c5fa14b29d1540a13b7d3ba13151b67fe719f820e0ef6d66aaaf"

	tests := []struct {
		name         string
		chartVersion string
		lockVersion  string
		wantError    bool
	}{
		{
			name:         "chart with the expected digest",
			chartVersion: "0.1.0",
			lockVersion:  "0.1.0",
			wantError:    false,
		},
		{
			name:         "ranged version but same resolved lock version",
			chartVersion: "^0.1.0",
			lockVersion:  "0.1.0",
			wantError:    true,
		},
		{
			name:         "ranged version resolved as higher version",
			chartVersion: "^0.1.0",
			lockVersion:  "0.1.2",
			wantError:    true,
		},
		{
			name:         "different version",
			chartVersion: "0.1.2",
			lockVersion:  "0.1.2",
			wantError:    true,
		},
		{
			name:         "different version with a range",
			chartVersion: "^0.1.2",
			lockVersion:  "0.1.2",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := []*chart.Dependency{
				{Name: "alpine", Version: tt.chartVersion, Repository: "http://localhost:8879/charts"},
			}
			lock := []*chart.Dependency{
				{Name: "alpine", Version: tt.lockVersion, Repository: "http://localhost:8879/charts"},
			}
			h, err := HashReq(req, lock)
			require.NoError(t, err)
			if !tt.wantError {
				assert.Equal(t, expect, h)
			} else {
				assert.NotEqual(t, expect, h, "Expected not %q, but same", expect)
			}
		})
	}
}

func TestGetLocalPath(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		chartpath string
		expect    string
		winExpect string
		err       bool
	}{
		{
			name:      "absolute path",
			repo:      "file:////",
			expect:    "/",
			winExpect: "\\",
		},
		{
			name:      "relative path",
			repo:      "file://../../testdata/chartpath/base",
			chartpath: "foo/bar",
			expect:    "testdata/chartpath/base",
			winExpect: "testdata\\chartpath\\base",
		},
		{
			name:      "current directory path",
			repo:      "../charts/localdependency",
			chartpath: "testdata/chartpath/charts",
			expect:    "testdata/chartpath/charts/localdependency",
			winExpect: "testdata\\chartpath\\charts\\localdependency",
		},
		{
			name:      "invalid local path",
			repo:      "file://testdata/nonexistent",
			chartpath: "testdata/chartpath",
			err:       true,
		},
		{
			name:      "invalid path under current directory",
			repo:      "charts/nonexistentdependency",
			chartpath: "testdata/chartpath/charts",
			err:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := GetLocalPath(tt.repo, tt.chartpath)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if runtime.GOOS == "windows" {
					assert.Equal(t, tt.winExpect, p, tt.name)
				} else {
					assert.Equal(t, tt.expect, p, tt.name)
				}
			}
		})
	}
}
