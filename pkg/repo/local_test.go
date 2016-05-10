package repo

import (
	"testing"
)

const testfile = "testdata/local-index.yaml"

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
				if len(e.Keywords) == 3 {
					nginx = true
				}
			}
		}
		if k == "alpine-1.0.0" {
			if e.Name == "alpine" {
				if len(e.Keywords) == 4 {
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
