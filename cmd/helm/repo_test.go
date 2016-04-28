package main

import (
	"testing"

	"github.com/kubernetes/helm/pkg/repo"
)

func TestRepoAdd(t *testing.T) {
	home := createTmpHome()
	helmHome = home
	if err := ensureHome(); err != nil {
		t.Errorf("%s", err)
	}

	testName := "test-name"
	testURL := "test-url"
	if err := insertRepoLine(testName, testURL); err != nil {
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

	if err := insertRepoLine(testName, testURL); err == nil {
		t.Errorf("Duplicate repository name was added")
	}

}
