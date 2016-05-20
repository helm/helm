package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

const testfile = "testdata/local-index.yaml"

var (
	testRepo = "test-repo"
)

func TestDownloadIndexFile(t *testing.T) {
	fileBytes, err := ioutil.ReadFile("testdata/local-index.yaml")
	if err != nil {
		t.Errorf("%#v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "binary/octet-stream")
		fmt.Fprintln(w, string(fileBytes))
	}))

	dirName, err := ioutil.TempDir("testdata", "tmp")
	path := filepath.Join(dirName, testRepo+"-index.yaml")
	if err := DownloadIndexFile(testRepo, ts.URL, path); err != nil {
		t.Errorf("%#v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("error finding created index file: %#v", err)
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Errorf("error reading index file: %#v", err)
	}

	var i IndexFile
	if err = yaml.Unmarshal(b, &i); err != nil {
		t.Errorf("error unmarshaling index file: %#v", err)
	}

	numEntries := len(i.Entries)
	if numEntries != 2 {
		t.Errorf("Expected 2 entries in index file but got %v", numEntries)
	}
	os.Remove(path)
	os.Remove(dirName)

}

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
