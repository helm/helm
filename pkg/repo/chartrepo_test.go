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

const (
	testRepository = "testdata/repository"
	testURL        = "http://example-charts.com"
)

func TestLoadChartRepository(t *testing.T) {
	r, err := NewChartRepository(&Entry{
		Name: testRepository,
		URL:  testURL,
	})
	if err != nil {
		t.Errorf("Problem creating chart repository from %s: %v", testRepository, err)
	}

	if err := r.Load(); err != nil {
		t.Errorf("Problem loading chart repository from %s: %v", testRepository, err)
	}

	paths := []string{
		filepath.Join(testRepository, "frobnitz-1.2.3.tgz"),
		filepath.Join(testRepository, "sprocket-1.1.0.tgz"),
		filepath.Join(testRepository, "sprocket-1.2.0.tgz"),
	}

	if r.Config.Name != testRepository {
		t.Errorf("Expected %s as Name but got %s", testRepository, r.Config.Name)
	}

	if !reflect.DeepEqual(r.ChartPaths, paths) {
		t.Errorf("Expected %#v but got %#v\n", paths, r.ChartPaths)
	}

	if r.Config.URL != testURL {
		t.Errorf("Expected url for chart repository to be %s but got %s", testURL, r.Config.URL)
	}
}

func TestIndex(t *testing.T) {
	r, err := NewChartRepository(&Entry{
		Name: testRepository,
		URL:  testURL,
	})
	if err != nil {
		t.Errorf("Problem creating chart repository from %s: %v", testRepository, err)
	}

	if err := r.Load(); err != nil {
		t.Errorf("Problem loading chart repository from %s: %v", testRepository, err)
	}

	err = r.Index()
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
	err = r.Index()
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
			{
				Metadata: &chart.Metadata{
					Name:    "sprocket",
					Version: "1.1.0",
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
