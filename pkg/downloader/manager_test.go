/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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
	"reflect"
	"testing"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm/helmpath"
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
	b := bytes.NewBuffer(nil)
	m := &Manager{
		Out:      b,
		HelmHome: helmpath.Home("testdata/helmhome"),
	}
	repos, err := m.loadChartRepositories()
	if err != nil {
		t.Fatal(err)
	}

	name := "alpine"
	version := "0.1.0"
	repoURL := "http://example.com/charts"

	churl, err := findChartURL(name, version, repoURL, repos)
	if err != nil {
		t.Fatal(err)
	}
	if churl != "https://kubernetes-charts.storage.googleapis.com/alpine-0.1.0.tgz" {
		t.Errorf("Unexpected URL %q", churl)
	}

}

func TestGetRepoNames(t *testing.T) {
	b := bytes.NewBuffer(nil)
	m := &Manager{
		Out:      b,
		HelmHome: helmpath.Home("testdata/helmhome"),
	}
	tests := []struct {
		name   string
		req    []*chartutil.Dependency
		expect map[string]string
		err    bool
	}{
		{
			name: "no repo definition failure",
			req: []*chartutil.Dependency{
				{Name: "oedipus-rex", Repository: "http://example.com/test"},
			},
			err: true,
		},
		{
			name: "no repo definition failure",
			req: []*chartutil.Dependency{
				{Name: "oedipus-rex", Repository: "http://example.com"},
			},
			expect: map[string]string{"oedipus-rex": "testing"},
		},
		{
			name: "repo from local path",
			req: []*chartutil.Dependency{
				{Name: "local-dep", Repository: "file://./testdata/signtest"},
			},
			expect: map[string]string{"local-dep": "file://./testdata/signtest"},
		},
		{
			name: "repo alias (alias:)",
			req: []*chartutil.Dependency{
				{Name: "oedipus-rex", Repository: "alias:testing"},
			},
			expect: map[string]string{"oedipus-rex": "testing"},
		},
		{
			name: "repo alias (@)",
			req: []*chartutil.Dependency{
				{Name: "oedipus-rex", Repository: "@testing"},
			},
			expect: map[string]string{"oedipus-rex": "testing"},
		},
	}

	for _, tt := range tests {
		l, err := m.getRepoNames(tt.req)
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
