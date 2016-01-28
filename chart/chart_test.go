package chart

import (
	"testing"

	"github.com/kubernetes/deployment-manager/log"
)

const (
	testfile    = "testdata/frobnitz/Chart.yaml"
	testdir     = "testdata/frobnitz/"
	testarchive = "testdata/frobnitz-0.0.1.tgz"
	testill     = "testdata/ill-1.2.3.tgz"
	testnochart = "testdata/nochart.tgz"
)

func init() {
	log.IsDebugging = true
}

func TestLoadDir(t *testing.T) {
	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected chart name to be 'frobnitz'. Got '%s'.", c.Chartfile().Name)
	}

	if c.Chartfile().Dependencies[0].Version != "^3" {
		d := c.Chartfile().Dependencies[0].Version
		t.Errorf("Expected dependency 0 to have version '^3'. Got '%s'.", d)
	}
}

func TestLoad(t *testing.T) {
	c, err := Load(testarchive)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	defer c.Close()

	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected name to be frobnitz, got %q", c.Chartfile().Name)
	}
}

func TestLoadIll(t *testing.T) {
	c, err := Load(testill)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	defer c.Close()

	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}
}

func TestLoadNochart(t *testing.T) {
	_, err := Load(testnochart)
	if err == nil {
		t.Error("Nochart should not have loaded at all.")
	}
}
