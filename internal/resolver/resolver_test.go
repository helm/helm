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
	"testing"

	"helm.sh/helm/v3/pkg/chart"
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
	r := New("testdata/chartpath", "testdata/repository")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := r.Resolve(tt.req, repoNames)
			if err != nil {
				if tt.err {
					return
				}
				t.Fatal(err)
			}

			if tt.err {
				t.Fatalf("Expected error in test %q", tt.name)
			}

			if h, err := HashReq(tt.req, tt.expect.Dependencies); err != nil {
				t.Fatal(err)
			} else if h != l.Digest {
				t.Errorf("%q: hashes don't match.", tt.name)
			}

			// Check fields.
			if len(l.Dependencies) != len(tt.req) {
				t.Errorf("%s: wrong number of dependencies in lock", tt.name)
			}
			d0 := l.Dependencies[0]
			e0 := tt.expect.Dependencies[0]
			if d0.Name != e0.Name {
				t.Errorf("%s: expected name %s, got %s", tt.name, e0.Name, d0.Name)
			}
			if d0.Repository != e0.Repository {
				t.Errorf("%s: expected repo %s, got %s", tt.name, e0.Repository, d0.Repository)
			}
			if d0.Version != e0.Version {
				t.Errorf("%s: expected version %s, got %s", tt.name, e0.Version, d0.Version)
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
			if err != nil {
				t.Fatal(err)
			}
			if !tt.wantError && expect != h {
				t.Errorf("Expected %q, got %q", expect, h)
			} else if tt.wantError && expect == h {
				t.Errorf("Expected not %q, but same", expect)
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
		err       bool
	}{
		{
			name:   "absolute path",
			repo:   "file:////tmp",
			expect: "/tmp",
		},
		{
			name:      "relative path",
			repo:      "file://../../testdata/chartpath/base",
			chartpath: "foo/bar",
			expect:    "testdata/chartpath/base",
		},
		{
			name:      "current directory path",
			repo:      "../charts/localdependency",
			chartpath: "testdata/chartpath/charts",
			expect:    "testdata/chartpath/charts/localdependency",
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
			if err != nil {
				if tt.err {
					return
				}
				t.Fatal(err)
			}
			if tt.err {
				t.Fatalf("Expected error in test %q", tt.name)
			}
			if p != tt.expect {
				t.Errorf("%q: expected %q, got %q", tt.name, tt.expect, p)
			}
		})
	}
}
