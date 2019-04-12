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
	"io/ioutil"
	"path"
	"testing"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/repo/repotest"
)

func TestResolve(t *testing.T) {
	cache, err := ioutil.TempDir("", "helm-resolver-test")
	if err != nil {
		t.Fatal(err)
	}

	registryClient := repo.NewClient(&repo.ClientOptions{
		Out:          ioutil.Discard,
		CacheRootDir: cache,
	})

	testRepo := repotest.NewServer()
	registryURL := testRepo.URL()

	versions := []string{"0.1.0", "0.2.0", "0.3.0"}

	for _, ver := range versions {
		ch := &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "alpine",
				Version: ver,
			},
		}

		ref, err := repo.ParseNameTag(path.Join(registryURL, ch.Metadata.Name), ch.Metadata.Version)
		if err != nil {
			t.Fatal(err)
		}

		if err := registryClient.SaveChart(ch, registryURL); err != nil {
			t.Fatal(err)
		}

		if err := registryClient.PushChart(ref); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name   string
		req    []*chart.Dependency
		expect *chart.Lock
		err    bool
	}{
		{
			name: "version failure",
			req: []*chart.Dependency{
				{Name: path.Join(registryURL, "oedipus-rex"), Version: ">a1"},
			},
			err: true,
		},
		{
			name: "cache index failure",
			req: []*chart.Dependency{
				{Name: path.Join(registryURL, "oedipus-rex"), Version: "1.0.0"},
			},
			err: true,
		},
		{
			name: "chart not found failure",
			req: []*chart.Dependency{
				{Name: path.Join(registryURL, "redis"), Version: "1.0.0"},
			},
			err: true,
		},
		{
			name: "constraint not satisfied failure",
			req: []*chart.Dependency{
				{Name: path.Join(registryURL, "alpine"), Version: ">=1.0.0"},
			},
			err: true,
		},
		{
			name: "valid lock",
			req: []*chart.Dependency{
				{Name: path.Join(registryURL, "alpine"), Version: ">=0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: path.Join(registryURL, "alpine"), Version: "0.3.0"},
				},
			},
		},
		{
			name: "exact lock",
			req: []*chart.Dependency{
				{Name: path.Join(registryURL, "alpine"), Version: "=0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: path.Join(registryURL, "alpine"), Version: "0.1.0"},
				},
			},
		},
		{
			name: "repo from valid local path",
			req: []*chart.Dependency{
				{Name: "file://../../../../cmd/helm/testdata/testcharts/signtest", Version: "0.1.0"},
			},
			expect: &chart.Lock{
				Dependencies: []*chart.Dependency{
					{Name: "file://../../../../cmd/helm/testdata/testcharts/signtest", Version: "0.1.0"},
				},
			},
		},
		{
			name: "repo from invalid local path",
			req: []*chart.Dependency{
				{Name: "file://../testdata/notexist", Version: "0.1.0"},
			},
			err: true,
		},
	}

	r := New("testdata/chartpath", registryClient)
	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashReq(tt.req)
			if err != nil {
				t.Fatal(err)
			}

			l, err := r.Resolve(tt.req, hash)
			if err != nil {
				if tt.err {
					return
				}
				t.Fatal(err)
			}

			if tt.err {
				t.Fatalf("Expected error")
			}

			if h, err := HashReq(tt.req); err != nil {
				t.Fatal(err)
			} else if h != l.Digest {
				t.Errorf("hashes don't match. expected '%s', got '%s'", h, l.Digest)
			}

			// Check fields.
			if len(l.Dependencies) != len(tt.req) {
				t.Errorf("wrong number of dependencies in lock. expected %d, got %d", len(tt.req), len(l.Dependencies))
			}
			d0 := l.Dependencies[0]
			e0 := tt.expect.Dependencies[0]
			if d0.Name != e0.Name {
				t.Errorf("expected name %s, got %s", e0.Name, d0.Name)
			}
			if d0.Version != e0.Version {
				t.Errorf("expected version %s, got %s", e0.Version, d0.Version)
			}
		})
	}
}

func TestHashReq(t *testing.T) {
	expect := "sha256:3aa1f5e784c4609f4db27c175e081e5ffab60ea4a27f87d889bb1ed273e49f75"
	req := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0"},
	}
	h, err := HashReq(req)
	if err != nil {
		t.Fatal(err)
	}
	if expect != h {
		t.Errorf("Expected %q, got %q", expect, h)
	}

	req = []*chart.Dependency{}
	h, err = HashReq(req)
	if err != nil {
		t.Fatal(err)
	}
	if expect == h {
		t.Errorf("Expected %q !=  %q", expect, h)
	}
}
