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
)

const testRepositoriesFile = "testdata/repositories.yaml"
const testRepository = "testdata/repository"
const testURL = "http://example-charts.com"

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

	entries := actual.Entries
	numEntries := len(entries)
	if numEntries != 2 {
		t.Errorf("Expected 2 charts to be listed in index file but got %v", numEntries)
	}

	timestamps := make(map[string]string)
	var empty time.Time
	for chartName, details := range entries {
		if details == nil {
			t.Errorf("Chart Entry is not filled out for %s", chartName)
		}

		if details.Created == empty.String() {
			t.Errorf("Created timestamp under %s chart entry is nil", chartName)
		}
		timestamps[chartName] = details.Created

		if details.Digest == "" {
			t.Errorf("Digest was not set for %s", chartName)
		}
	}

	if err = cr.Index(); err != nil {
		t.Errorf("Error performing index the second time: %v\n", err)
	}
	second, err := LoadIndexFile(tempIndexPath)
	if err != nil {
		t.Errorf("Error loading index file second time: %#v\n", err)
	}

	for chart, created := range timestamps {
		v, ok := second.Entries[chart]
		if !ok {
			t.Errorf("Expected %s chart entry in index file but did not find it", chart)
		}
		if v.Created != created {
			t.Errorf("Expected Created timestamp to be %s, but got %s for chart %s", created, v.Created, chart)
		}
		// Created manually since we control the input of the test
		expectedURL := testURL + "/" + chart + ".tgz"
		if v.URL != expectedURL {
			t.Errorf("Expected url in entry to be %s but got %s for chart: %s", expectedURL, v.URL, chart)
		}
	}
}

func TestLoadRepositoriesFile(t *testing.T) {
	rf, err := LoadRepositoriesFile(testRepositoriesFile)
	if err != nil {
		t.Errorf(testRepositoriesFile + " could not be loaded: " + err.Error())
	}
	expected := map[string]string{"best-charts-ever": "http://best-charts-ever.com",
		"okay-charts": "http://okay-charts.org", "example123": "http://examplecharts.net/charts/123"}

	numOfRepositories := len(rf.Repositories)
	expectedNumOfRepositories := 3
	if numOfRepositories != expectedNumOfRepositories {
		t.Errorf("Expected %v repositories but only got %v", expectedNumOfRepositories, numOfRepositories)
	}

	for expectedRepo, expectedURL := range expected {
		actual, ok := rf.Repositories[expectedRepo]
		if !ok {
			t.Errorf("Expected repository: %v but was not found", expectedRepo)
		}

		if expectedURL != actual {
			t.Errorf("Expected url %s for the %s repository but got %s ", expectedURL, expectedRepo, actual)
		}
	}
}
