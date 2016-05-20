package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const testfile = "testdata/local-index.yaml"
const testRepositoriesFile = "testdata/repositories.yaml"
const testRepository = "testdata/repository"
const testURL = "http://example-charts.com"

func TestLoadIndexFile(t *testing.T) {
	cf, err := LoadIndexFile(testfile)
	if err != nil {
		t.Errorf("Failed to load index file: %s", err)
	}
	if len(cf.Entries) != 2 {
		t.Errorf("Expected 2 entries in the index file, but got %d", len(cf.Entries))
	}
	nginx := false
	alpine := false
	for k, e := range cf.Entries {
		if k == "nginx-0.1.0" {
			if e.Name == "nginx" {
				if len(e.Chartfile.Keywords) == 3 {
					nginx = true
				}
			}
		}
		if k == "alpine-1.0.0" {
			if e.Name == "alpine" {
				if len(e.Chartfile.Keywords) == 4 {
					alpine = true
				}
			}
		}
	}
	if !nginx {
		t.Errorf("nginx entry was not decoded properly")
	}
	if !alpine {
		t.Errorf("alpine entry was not decoded properly")
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
	if err != nil {
		t.Errorf("Error loading index file %v", err)
	}

	entries := actual.Entries
	numEntries := len(entries)
	if numEntries != 2 {
		t.Errorf("Expected 2 charts to be listed in index file but got %v", numEntries)
	}

	var empty time.Time
	for chartName, details := range entries {
		if details == nil {
			t.Errorf("Chart Entry is not filled out for %s", chartName)
		}

		if details.Created == empty.String() {
			t.Errorf("Created timestamp under %s chart entry is nil", chartName)
		}

		if details.Digest == "" {
			t.Errorf("Digest was not set for %s", chartName)
		}
	}

	//TODO: test update case

	os.Remove(tempIndexPath) // clean up
}
