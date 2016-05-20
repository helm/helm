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
