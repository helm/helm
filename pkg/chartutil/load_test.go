package chartutil

import (
	"testing"

	"github.com/kubernetes/helm/pkg/proto/hapi/chart"
)

func TestLoadDir(t *testing.T) {
	c, err := Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyFrobnitz(t, c)
	verifyChart(t, c)
}

func TestLoadFile(t *testing.T) {
	c, err := Load("testdata/frobnitz-1.2.3.tgz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyFrobnitz(t, c)
	verifyChart(t, c)
}

func verifyChart(t *testing.T, c *chart.Chart) {
	if len(c.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(c.Templates))
	}

	if len(c.Files) != 5 {
		t.Errorf("Expected 5 extra files, got %d", len(c.Files))
		for _, n := range c.Files {
			t.Logf("\t%s", n.TypeUrl)
		}
	}

	if len(c.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d (%v)", len(c.Dependencies), c.Dependencies)
		for _, d := range c.Dependencies {
			t.Logf("\tSubchart: %s\n", d.Metadata.Name)
		}
	}

	expect := map[string]map[string]string{
		"alpine": map[string]string{
			"version": "0.1.0",
		},
		"mariner": map[string]string{
			"version": "4.3.2",
		},
	}

	for _, dep := range c.Dependencies {
		exp, ok := expect[dep.Metadata.Name]
		if !ok {
			t.Fatalf("Unknown dependency %s", dep.Metadata.Name)
		}
		if exp["version"] != dep.Metadata.Version {
			t.Errorf("Expected %s version %s, got %s", dep.Metadata.Name, exp["version"], dep.Metadata.Version)
		}
	}

}

func verifyFrobnitz(t *testing.T, c *chart.Chart) {
	verifyChartfile(t, c.Metadata)

	if len(c.Templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(c.Templates))
	}

	if c.Templates[0].Name != "templates/template.tpl" {
		t.Errorf("Unexpected template: %s", c.Templates[0].Name)
	}

	if len(c.Templates[0].Data) == 0 {
		t.Error("No template data.")
	}
}
