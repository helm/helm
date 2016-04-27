package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestEnsureHome(t *testing.T) {
	home := createTmpHome()
	if err := ensureHome(home); err != nil {
		t.Errorf("%s", err)
	}

	expectedDirs := []string{home, cachePath, localRepoPath}
	for _, dir := range expectedDirs {
		if fi, err := os.Stat(dir); err != nil {
			t.Errorf("%s", err)
		} else if !fi.IsDir() {
			t.Errorf("%s is not a directory", fi)
		}
	}

	if fi, err := os.Stat(repositoriesFilePath); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}

	if fi, err := os.Stat(localCacheFilePath); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}
}

func createTmpHome() string {
	tmpHome, _ := ioutil.TempDir("", "helm_home")
	defer os.Remove(tmpHome)
	return tmpHome
}
