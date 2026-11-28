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
	"slices"
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

// fakeOCIRegistry is a minimal OCI registry serving the tag list endpoint and
// HEAD requests for manifests, which is all that OCI dependency resolution needs.
type fakeOCIRegistry struct {
	tags []string
	// headStatus, when set, is returned for every manifest HEAD request
	// instead of answering from tags.
	headStatus int
}

func (f *fakeOCIRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags/list"):
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"testrepo/mychart","tags":["` + strings.Join(f.tags, `","`) + `"]}`))
	case r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/manifests/"):
		if f.headStatus != 0 {
			w.WriteHeader(f.headStatus)
			return
		}
		tag := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if !slices.Contains(f.tags, tag) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("a", 64))
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestResolveOCIVersions(t *testing.T) {
	tests := []struct {
		name       string
		tags       []string
		version    string
		want       string
		err        bool
		headStatus int
	}{
		{
			name:    "strict version is locked as is",
			tags:    []string{"1.2.0", "1.2.5"},
			version: "1.2.5",
			want:    "1.2.5",
		},
		{
			name:       "partial version fails when the literal tag check is forbidden",
			tags:       []string{"1.2", "1.2.0", "1.2.5"},
			version:    "1.2",
			err:        true,
			headStatus: http.StatusForbidden,
		},
		{
			name:    "partial version resolves to the highest matching tag",
			tags:    []string{"1.1.0", "1.2.0", "1.2.5", "1.3.0"},
			version: "1.2",
			want:    "1.2.5",
		},
		{
			name:    "partial version prefers an existing literal tag",
			tags:    []string{"1.2", "1.2.0", "1.2.5"},
			version: "1.2",
			want:    "1.2",
		},
		{
			name:    "major version resolves to the highest matching tag",
			tags:    []string{"1.2.0", "1.2.5", "2.0.0"},
			version: "1",
			want:    "1.2.5",
		},
		{
			name:    "major version prefers an existing literal tag",
			tags:    []string{"1", "1.2.0", "2.0.0"},
			version: "1",
			want:    "1",
		},
		{
			name:    "range resolves to the highest matching tag",
			tags:    []string{"1.2.0", "1.2.5", "1.3.0"},
			version: "~1.2",
			want:    "1.2.5",
		},
		{
			name:    "invalid version fails",
			tags:    []string{"1.2.0"},
			version: ">a1",
			err:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(&fakeOCIRegistry{tags: tt.tags, headStatus: tt.headStatus})
			defer srv.Close()
			host := strings.TrimPrefix(srv.URL, "http://")

			registryClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
			require.NoError(t, err)
			r := New("testdata/chartpath", "testdata/repository", registryClient)

			req := []*chart.Dependency{
				{Name: "mychart", Repository: "oci://" + host + "/testrepo", Version: tt.version},
			}
			l, err := r.Resolve(req, map[string]string{"mychart": "testrepo"})
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, l.Dependencies, 1)
			assert.Equal(t, tt.want, l.Dependencies[0].Version)
		})
	}
}
