/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chartutil

import (
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestLoadDir(t *testing.T) {
	c, err := Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyRequirements(t, c)
}

func TestLoadFile(t *testing.T) {
	c, err := Load("testdata/frobnitz-1.2.3.tgz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyRequirements(t, c)
}

func verifyChart(t *testing.T, c *chart.Chart) {
	if c.Metadata.Name == "" {
		t.Fatalf("No chart metadata found on %v", c)
	}
	t.Logf("Verifying chart %s", c.Metadata.Name)
	if len(c.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(c.Templates))
	}

	numfiles := 8
	if len(c.Files) != numfiles {
		t.Errorf("Expected %d extra files, got %d", numfiles, len(c.Files))
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
		"alpine": {
			"version": "0.1.0",
		},
		"mariner": {
			"version": "4.3.2",
		},
	}

	for _, dep := range c.Dependencies {
		if dep.Metadata == nil {
			t.Fatalf("expected metadata on dependency: %v", dep)
		}
		exp, ok := expect[dep.Metadata.Name]
		if !ok {
			t.Fatalf("Unknown dependency %s", dep.Metadata.Name)
		}
		if exp["version"] != dep.Metadata.Version {
			t.Errorf("Expected %s version %s, got %s", dep.Metadata.Name, exp["version"], dep.Metadata.Version)
		}
	}

}

func verifyRequirements(t *testing.T, c *chart.Chart) {
	r, err := LoadRequirements(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Dependencies) != 2 {
		t.Errorf("Expected 2 requirements, got %d", len(r.Dependencies))
	}
	tests := []*Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := r.Dependencies[i]
		if d.Name != tt.Name {
			t.Errorf("Expected dependency named %q, got %q", tt.Name, d.Name)
		}
		if d.Version != tt.Version {
			t.Errorf("Expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, d.Version)
		}
		if d.Repository != tt.Repository {
			t.Errorf("Expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, d.Repository)
		}
	}
}
func verifyRequirementsLock(t *testing.T, c *chart.Chart) {
	r, err := LoadRequirementsLock(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Dependencies) != 2 {
		t.Errorf("Expected 2 requirements, got %d", len(r.Dependencies))
	}
	tests := []*Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := r.Dependencies[i]
		if d.Name != tt.Name {
			t.Errorf("Expected dependency named %q, got %q", tt.Name, d.Name)
		}
		if d.Version != tt.Version {
			t.Errorf("Expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, d.Version)
		}
		if d.Repository != tt.Repository {
			t.Errorf("Expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, d.Repository)
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
