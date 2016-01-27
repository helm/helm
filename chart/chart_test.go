package chart

import (
	"testing"
)

const testfile = "../testdata/test-Chart.yaml"
const testchart = "../testdata/charts/kitchensink"

func TestLoad(t *testing.T) {
	c, err := Load(testchart)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	if c.Chartfile.Name != "kitchensink" {
		t.Errorf("Expected chart name to be 'kitchensink'. Got '%s'.", c.Chartfile.Name)
	}
	if c.Chartfile.Dependencies[0].Version != "~10.21" {
		d := c.Chartfile.Dependencies[0].Version
		t.Errorf("Expected dependency 0 to have version '~10.21'. Got '%s'.", d)
	}

	if len(c.Kind["Pod"]) != 3 {
		t.Errorf("Expected 3 pods, got %d", len(c.Kind["Pod"]))
	}

	if len(c.Kind["ReplicationController"]) == 0 {
		t.Error("No RCs found")
	}
	if len(c.Kind["Namespace"]) == 0 {
		t.Errorf("No namespaces found")
	}

	if len(c.Kind["Secret"]) == 0 {
		t.Error("Is it secret? Is it safe? NO!")
	}

	if len(c.Kind["PersistentVolume"]) == 0 {
		t.Errorf("No volumes.")
	}

	if len(c.Kind["Service"]) == 0 {
		t.Error("No service. Just like [insert mobile provider name here]")
	}
}

func TestLoadChart(t *testing.T) {
	f, err := LoadChartfile(testfile)
	if err != nil {
		t.Errorf("Error loading %s: %s", testfile, err)
	}

	if f.Name != "alpine-pod" {
		t.Errorf("Expected alpine-pod, got %s", f.Name)
	}

	if len(f.Maintainers) != 2 {
		t.Errorf("Expected 2 maintainers, got %d", len(f.Maintainers))
	}

	if len(f.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(f.Dependencies))
	}

	if f.Dependencies[1].Name != "bar" {
		t.Errorf("Expected second dependency to be bar: %q", f.Dependencies[1].Name)
	}

	if f.PreInstall["mykeys"] != "generate-keypair foo" {
		t.Errorf("Expected map value for mykeys.")
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

	d := f.Dependencies[1]
	if d.VersionOK("1.0.0") {
		t.Errorf("1.0.0 should have been marked out of range")
	}

	if !d.VersionOK("1.2.3") {
		t.Errorf("Version 1.2.3 should have been marked in-range")
	}

}

func TestUnknownKinds(t *testing.T) {
	known := []string{"Pod"}
	c, err := Load(testchart)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	unknown := c.UnknownKinds(known)
	if len(unknown) < 5 {
		t.Errorf("Expected at least 5 unknown chart types, got %d.", len(unknown))
	}

	for _, k := range unknown {
		if k == "Pod" {
			t.Errorf("Pod is not an unknown kind.")
		}
	}
}
