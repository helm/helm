package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubernetes/helm/pkg/repo"
)

var (
	testName = "test-name"
	testURL  = "test-url"
)

func TestRepoAdd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "OK")
	}))

	helmHome, _ = ioutil.TempDir("", "helm_home")
	defer os.Remove(helmHome)
	os.Mkdir(filepath.Join(helmHome, repositoryDir), 0755)
	os.Mkdir(cacheDirectory(), 0755)

	if err := ioutil.WriteFile(repositoriesFile(), []byte("example-repo: http://exampleurl.com"), 0666); err != nil {
		t.Errorf("%#v", err)
	}

	if err := addRepository(testName, ts.URL); err != nil {
		t.Errorf("%s", err)
	}

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		t.Errorf("%s", err)
	}
	_, ok := f.Repositories[testName]
	if !ok {
		t.Errorf("%s was not successfully inserted into %s", testName, repositoriesFile())
	}

	if err := insertRepoLine(testName, ts.URL); err == nil {
		t.Errorf("Duplicate repository name was added")
	}

}

func TestRepoRemove(t *testing.T) {
	home := createTmpHome()
	helmHome = home
	if err := ensureHome(); err != nil {
		t.Errorf("%s", err)
	}

	if err := removeRepoLine(testName); err == nil {
		t.Errorf("Expected error removing %s, but did not get one.", testName)
	}

	if err := insertRepoLine(testName, testURL); err != nil {
		t.Errorf("%s", err)
	}

	if err := removeRepoLine(testName); err != nil {
		t.Errorf("Error removing %s from repositories", testName)
	}

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		t.Errorf("%s", err)
	}

	if _, ok := f.Repositories[testName]; ok {
		t.Errorf("%s was not successfully removed from repositories list", testName)
	}
}
