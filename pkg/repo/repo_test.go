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

package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

const testRepositoriesFile = "testdata/repositories.yaml"
const testRepository = "testdata/repository"
const testURL = "http://example-charts.com"

func TestRepoFile(t *testing.T) {
	rf := NewRepoFile()
	rf.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)

	if len(rf.Repositories) != 2 {
		t.Fatal("Expected 2 repositories")
	}

	if rf.Has("nosuchrepo") {
		t.Error("Found nonexistent repo")
	}
	if !rf.Has("incubator") {
		t.Error("incubator repo is missing")
	}

	stable := rf.Repositories[0]
	if stable.Name != "stable" {
		t.Error("stable is not named stable")
	}
	if stable.URL != "https://example.com/stable/charts" {
		t.Error("Wrong URL for stable")
	}
	if stable.Cache != "stable-index.yaml" {
		t.Error("Wrong cache name for stable")
	}
}

func TestLoadRepositoriesFile(t *testing.T) {
	expects := NewRepoFile()
	expects.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)

	repofile, err := LoadRepositoriesFile(testRepositoriesFile)
	if err != nil {
		t.Errorf("%q could not be loaded: %s", testRepositoriesFile, err)
	}

	if len(expects.Repositories) != len(repofile.Repositories) {
		t.Fatalf("Unexpected repo data: %#v", repofile.Repositories)
	}

	for i, expect := range expects.Repositories {
		got := repofile.Repositories[i]
		if expect.Name != got.Name {
			t.Errorf("Expected name %q, got %q", expect.Name, got.Name)
		}
		if expect.URL != got.URL {
			t.Errorf("Expected url %q, got %q", expect.URL, got.URL)
		}
		if expect.Cache != got.Cache {
			t.Errorf("Expected cache %q, got %q", expect.Cache, got.Cache)
		}
	}
}

func TestLoadPreV1RepositoriesFile(t *testing.T) {
	r, err := LoadRepositoriesFile("testdata/old-repositories.yaml")
	if err != nil && err != ErrRepoOutOfDate {
		t.Fatal(err)
	}
	if len(r.Repositories) != 3 {
		t.Fatalf("Expected 3 repos: %#v", r)
	}

	// Because they are parsed as a map, we lose ordering.
	found := false
	for _, rr := range r.Repositories {
		if rr.Name == "best-charts-ever" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the best charts ever. Got %#v", r.Repositories)
	}
}

func TestLoadChartRepository(t *testing.T) {
	cr, err := LoadChartRepository(testRepository, testURL)
	if err != nil {
		t.Errorf("Problem loading chart repository from %s: %v", testRepository, err)
	}

	paths := []string{filepath.Join(testRepository, "frobnitz-1.2.3.tgz"), filepath.Join(testRepository, "sprocket-1.2.0.tgz")}

	if cr.RootPath != testRepository {
		t.Errorf("Expected %s as RootPath but got %s", testRepository, cr.RootPath)
	}

	if !reflect.DeepEqual(cr.ChartPaths, paths) {
		t.Errorf("Expected %#v but got %#v\n", paths, cr.ChartPaths)
	}

	if cr.URL != testURL {
		t.Errorf("Expected url for chart repository to be %s but got %s", testURL, cr.URL)
	}
}

func TestIndex(t *testing.T) {
	cr, err := LoadChartRepository(testRepository, testURL)
	if err != nil {
		t.Errorf("Problem loading chart repository from %s: %v", testRepository, err)
	}

	err = cr.Index()
	if err != nil {
		t.Errorf("Error performing index: %v\n", err)
	}

	tempIndexPath := filepath.Join(testRepository, indexPath)
	actual, err := LoadIndexFile(tempIndexPath)
	defer os.Remove(tempIndexPath) // clean up
	if err != nil {
		t.Errorf("Error loading index file %v", err)
	}
	verifyIndex(t, actual)

	// Re-index and test again.
	err = cr.Index()
	if err != nil {
		t.Errorf("Error performing re-index: %s\n", err)
	}
	second, err := LoadIndexFile(tempIndexPath)
	if err != nil {
		t.Errorf("Error re-loading index file %v", err)
	}
	verifyIndex(t, second)
}

func verifyIndex(t *testing.T, actual *IndexFile) {

	var empty time.Time
	if actual.Generated == empty {
		t.Errorf("Generated should be greater than 0: %s", actual.Generated)
	}

	if actual.APIVersion != APIVersionV1 {
		t.Error("Expected v1 API")
	}

	entries := actual.Entries
	if numEntries := len(entries); numEntries != 2 {
		t.Errorf("Expected 2 charts to be listed in index file but got %v", numEntries)
	}

	expects := map[string]ChartVersions{
		"frobnitz": {
			{
				Metadata: &chart.Metadata{
					Name:    "frobnitz",
					Version: "1.2.3",
				},
			},
		},
		"sprocket": {
			{
				Metadata: &chart.Metadata{
					Name:    "sprocket",
					Version: "1.2.0",
				},
			},
		},
	}

	for name, versions := range expects {
		got, ok := entries[name]
		if !ok {
			t.Errorf("Could not find %q entry", name)
			continue
		}
		if len(versions) != len(got) {
			t.Errorf("Expected %d versions, got %d", len(versions), len(got))
			continue
		}
		for i, e := range versions {
			g := got[i]
			if e.Name != g.Name {
				t.Errorf("Expected %q, got %q", e.Name, g.Name)
			}
			if e.Version != g.Version {
				t.Errorf("Expected %q, got %q", e.Version, g.Version)
			}
			if len(g.Keywords) != 3 {
				t.Error("Expected 3 keyrwords.")
			}
			if len(g.Maintainers) != 2 {
				t.Error("Expected 2 maintainers.")
			}
			if g.Created == empty {
				t.Error("Expected created to be non-empty")
			}
			if g.Description == "" {
				t.Error("Expected description to be non-empty")
			}
			if g.Home == "" {
				t.Error("Expected home to be non-empty")
			}
			if g.Digest == "" {
				t.Error("Expected digest to be non-empty")
			}
			if len(g.URLs) != 1 {
				t.Error("Expected exactly 1 URL")
			}
		}
	}
}
