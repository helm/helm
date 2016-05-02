package main

import (
	"testing"

	"github.com/kubernetes/helm/pkg/repo"
)

var (
	testName = "test-name"
	testURL  = "test-url"
)

func TestRepoAdd(t *testing.T) {
	home := createTmpHome()
	helmHome = home
	if err := ensureHome(); err != nil {
		t.Errorf("%s", err)
	}

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
