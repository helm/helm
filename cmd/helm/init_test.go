package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestEnsureHome(t *testing.T) {
	home := CreateTmpHome()
	if err := EnsureHome(home); err != nil {
		t.Errorf("%s", err)
	}

	dirs := []string{home, CacheDirectory(home)}
	for _, dir := range dirs {
		if fi, err := os.Stat(dir); err != nil {
			t.Errorf("%s", err)
		} else if !fi.IsDir() {
			t.Errorf("%s is not a directory", fi)
		}
	}

	if fi, err := os.Stat(RepositoriesFile(home)); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}

}

func CreateTmpHome() string {
	tmpHome, _ := ioutil.TempDir("", "helm_home")
	defer os.Remove(tmpHome)
	return tmpHome
}
