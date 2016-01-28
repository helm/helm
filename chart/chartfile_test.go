package chart

import (
	"testing"
)

func TestLoadChartfile(t *testing.T) {
	f, err := LoadChartfile(testfile)
	if err != nil {
		t.Errorf("Failed to open %s: %s", testfile, err)
		return
	}

	if len(f.Environment[0].Extensions) != 2 {
		t.Errorf("Expected two extensions, got %d", len(f.Environment[0].Extensions))
	}

	if f.Name != "frobnitz" {
		t.Errorf("Expected frobnitz, got %s", f.Name)
	}

	if len(f.Maintainers) != 2 {
		t.Errorf("Expected 2 maintainers, got %d", len(f.Maintainers))
	}

	if len(f.Dependencies) != 1 {
		t.Errorf("Expected 2 dependencies, got %d", len(f.Dependencies))
	}

	if f.Dependencies[0].Name != "thingerbob" {
		t.Errorf("Expected second dependency to be thingerbob: %q", f.Dependencies[0].Name)
	}

	if f.Source[0] != "https://example.com/helm" {
		t.Errorf("Expected https://example.com/helm, got %s", f.Source)
	}
}

func TestVersionOK(t *testing.T) {
	f, err := LoadChartfile(testfile)
	if err != nil {
		t.Errorf("Error loading %s: %s", testfile, err)
	}

	// These are canaries. The SemVer package exhuastively tests the
	// various  permutations. This will alert us if we wired it up
	// incorrectly.

	d := f.Dependencies[0]
	if d.VersionOK("1.0.0") {
		t.Errorf("1.0.0 should have been marked out of range")
	}

	if !d.VersionOK("3.2.3") {
		t.Errorf("Version 3.2.3 should have been marked in-range")
	}

}
