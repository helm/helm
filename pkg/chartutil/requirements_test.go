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
	"sort"
	"testing"

	"strconv"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/version"
)

func TestLoadRequirements(t *testing.T) {
	c, err := loader.Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyRequirements(t, c)
}

func TestLoadRequirementsLock(t *testing.T) {
	c, err := loader.Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyRequirementsLock(t, c)
}

func TestRequirementsEnabled(t *testing.T) {
	tests := []struct {
		name string
		v    []byte
		e    []string // expected charts including duplicates in alphanumeric order
	}{{
		"tags with no effect",
		[]byte("tags:\n  nothinguseful: false\n\n"),
		[]string{"parentchart", "subchart1", "subcharta", "subchartb"},
	}, {
		"tags with no effect",
		[]byte("tags:\n  nothinguseful: false\n\n"),
		[]string{"parentchart", "subchart1", "subcharta", "subchartb"},
	}, {
		"tags disabling a group",
		[]byte("tags:\n  front-end: false\n\n"),
		[]string{"parentchart"},
	}, {
		"tags disabling a group and enabling a different group",
		[]byte("tags:\n  front-end: false\n\n  back-end: true\n"),
		[]string{"parentchart", "subchart2", "subchartb", "subchartc"},
	}, {
		"tags disabling only children, children still enabled since tag front-end=true in values.yaml",
		[]byte("tags:\n  subcharta: false\n\n  subchartb: false\n"),
		[]string{"parentchart", "subchart1", "subcharta", "subchartb"},
	}, {
		"tags disabling all parents/children with additional tag re-enabling a parent",
		[]byte("tags:\n  front-end: false\n\n  subchart1: true\n\n  back-end: false\n"),
		[]string{"parentchart", "subchart1"},
	}, {
		"tags with no effect",
		[]byte("subchart1:\n  nothinguseful: false\n\n"),
		[]string{"parentchart", "subchart1", "subcharta", "subchartb"},
	}, {
		"conditions enabling the parent charts, but back-end (b, c) is still disabled via values.yaml",
		[]byte("subchart1:\n  enabled: true\nsubchart2:\n  enabled: true\n"),
		[]string{"parentchart", "subchart1", "subchart2", "subcharta", "subchartb"},
	}, {
		"conditions disabling the parent charts, effectively disabling children",
		[]byte("subchart1:\n  enabled: false\nsubchart2:\n  enabled: false\n"),
		[]string{"parentchart"},
	}, {
		"conditions a child using the second condition path of child's condition",
		[]byte("subchart1:\n  subcharta:\n    enabled: false\n"),
		[]string{"parentchart", "subchart1", "subchartb"},
	}, {
		"tags enabling a parent/child group with condition disabling one child",
		[]byte("subchartc:\n  enabled: false\ntags:\n  back-end: true\n"),
		[]string{"parentchart", "subchart1", "subchart2", "subcharta", "subchartb", "subchartb"},
	}, {
		"tags will not enable a child if parent is explicitly disabled with condition",
		[]byte("subchart1:\n  enabled: false\ntags:\n  front-end: true\n"),
		[]string{"parentchart"},
	}}

	for _, tc := range tests {
		c, err := loader.Load("testdata/subpop")
		if err != nil {
			t.Fatalf("Failed to load testdata: %s", err)
		}
		t.Run(tc.name, func(t *testing.T) {
			verifyRequirementsEnabled(t, c, tc.v, tc.e)
		})
	}
}

func verifyRequirementsEnabled(t *testing.T, c *chart.Chart, v []byte, e []string) {
	if err := ProcessRequirementsEnabled(c, v); err != nil {
		t.Errorf("Error processing enabled requirements %v", err)
	}

	out := extractCharts(c, nil)
	// build list of chart names
	var p []string
	for _, r := range out {
		p = append(p, r.Name())
	}
	//sort alphanumeric and compare to expectations
	sort.Strings(p)
	if len(p) != len(e) {
		t.Errorf("Error slice lengths do not match got %v, expected %v", len(p), len(e))
		return
	}
	for i := range p {
		if p[i] != e[i] {
			t.Errorf("Error slice values do not match got %v, expected %v", p[i], e[i])
		}
	}
}

// extractCharts recursively searches chart dependencies returning all charts found
func extractCharts(c *chart.Chart, out []*chart.Chart) []*chart.Chart {
	if len(c.Name()) > 0 {
		out = append(out, c)
	}
	for _, d := range c.Dependencies() {
		out = extractCharts(d, out)
	}
	return out
}

func TestProcessRequirementsImportValues(t *testing.T) {
	c, err := loader.Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	e := make(map[string]string)

	e["imported-chart1.SC1bool"] = "true"
	e["imported-chart1.SC1float"] = "3.14"
	e["imported-chart1.SC1int"] = "100"
	e["imported-chart1.SC1string"] = "dollywood"
	e["imported-chart1.SC1extra1"] = "11"
	e["imported-chart1.SPextra1"] = "helm rocks"
	e["imported-chart1.SC1extra1"] = "11"

	e["imported-chartA.SCAbool"] = "false"
	e["imported-chartA.SCAfloat"] = "3.1"
	e["imported-chartA.SCAint"] = "55"
	e["imported-chartA.SCAstring"] = "jabba"
	e["imported-chartA.SPextra3"] = "1.337"
	e["imported-chartA.SC1extra2"] = "1.337"
	e["imported-chartA.SCAnested1.SCAnested2"] = "true"

	e["imported-chartA-B.SCAbool"] = "false"
	e["imported-chartA-B.SCAfloat"] = "3.1"
	e["imported-chartA-B.SCAint"] = "55"
	e["imported-chartA-B.SCAstring"] = "jabba"

	e["imported-chartA-B.SCBbool"] = "true"
	e["imported-chartA-B.SCBfloat"] = "7.77"
	e["imported-chartA-B.SCBint"] = "33"
	e["imported-chartA-B.SCBstring"] = "boba"
	e["imported-chartA-B.SPextra5"] = "k8s"
	e["imported-chartA-B.SC1extra5"] = "tiller"

	e["overridden-chart1.SC1bool"] = "false"
	e["overridden-chart1.SC1float"] = "3.141592"
	e["overridden-chart1.SC1int"] = "99"
	e["overridden-chart1.SC1string"] = "pollywog"
	e["overridden-chart1.SPextra2"] = "42"

	e["overridden-chartA.SCAbool"] = "true"
	e["overridden-chartA.SCAfloat"] = "41.3"
	e["overridden-chartA.SCAint"] = "808"
	e["overridden-chartA.SCAstring"] = "jaberwocky"
	e["overridden-chartA.SPextra4"] = "true"

	e["overridden-chartA-B.SCAbool"] = "true"
	e["overridden-chartA-B.SCAfloat"] = "41.3"
	e["overridden-chartA-B.SCAint"] = "808"
	e["overridden-chartA-B.SCAstring"] = "jaberwocky"
	e["overridden-chartA-B.SCBbool"] = "false"
	e["overridden-chartA-B.SCBfloat"] = "1.99"
	e["overridden-chartA-B.SCBint"] = "77"
	e["overridden-chartA-B.SCBstring"] = "jango"
	e["overridden-chartA-B.SPextra6"] = "111"
	e["overridden-chartA-B.SCAextra1"] = "23"
	e["overridden-chartA-B.SCBextra1"] = "13"
	e["overridden-chartA-B.SC1extra6"] = "77"

	// `exports` style
	e["SCBexported1B"] = "1965"
	e["SC1extra7"] = "true"
	e["SCBexported2A"] = "blaster"
	e["global.SC1exported2.all.SC1exported3"] = "SC1expstr"

	verifyRequirementsImportValues(t, c, e)
}

func verifyRequirementsImportValues(t *testing.T, c *chart.Chart, e map[string]string) {
	if err := ProcessRequirementsImportValues(c); err != nil {
		t.Fatalf("Error processing import values requirements %v", err)
	}
	cc, err := ReadValues(c.Values)
	if err != nil {
		t.Fatalf("Error reading import values %v", err)
	}
	for kk, vv := range e {
		pv, err := cc.PathValue(kk)
		if err != nil {
			t.Fatalf("Error retrieving import values table %v %v", kk, err)
			return
		}

		switch pv.(type) {
		case float64:
			s := strconv.FormatFloat(pv.(float64), 'f', -1, 64)
			if s != vv {
				t.Errorf("Failed to match imported float value %v with expected %v", s, vv)
				return
			}
		case bool:
			b := strconv.FormatBool(pv.(bool))
			if b != vv {
				t.Errorf("Failed to match imported bool value %v with expected %v", b, vv)
				return
			}
		default:
			if pv.(string) != vv {
				t.Errorf("Failed to match imported string value %q with expected %q", pv, vv)
				return
			}
		}
	}
}

func TestGetAliasDependency(t *testing.T) {
	c, err := loader.Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	req := c.Requirements

	if len(req.Dependencies) == 0 {
		t.Fatalf("There are no requirements to test")
	}

	// Success case
	aliasChart := getAliasDependency(c.Dependencies(), req.Dependencies[0])
	if aliasChart == nil {
		t.Fatalf("Failed to get dependency chart for alias %s", req.Dependencies[0].Name)
	}
	if req.Dependencies[0].Alias != "" {
		if aliasChart.Name() != req.Dependencies[0].Alias {
			t.Fatalf("Dependency chart name should be %s but got %s", req.Dependencies[0].Alias, aliasChart.Name())
		}
	} else if aliasChart.Name() != req.Dependencies[0].Name {
		t.Fatalf("Dependency chart name should be %s but got %s", req.Dependencies[0].Name, aliasChart.Name())
	}

	if req.Dependencies[0].Version != "" {
		if !version.IsCompatibleRange(req.Dependencies[0].Version, aliasChart.Metadata.Version) {
			t.Fatalf("Dependency chart version is not in the compatible range")
		}
	}

	// Failure case
	req.Dependencies[0].Name = "something-else"
	if aliasChart := getAliasDependency(c.Dependencies(), req.Dependencies[0]); aliasChart != nil {
		t.Fatalf("expected no chart but got %s", aliasChart.Name())
	}

	req.Dependencies[0].Version = "something else which is not in the compatible range"
	if version.IsCompatibleRange(req.Dependencies[0].Version, aliasChart.Metadata.Version) {
		t.Fatalf("Dependency chart version which is not in the compatible range should cause a failure other than a success ")
	}
}

func TestDependentChartAliases(t *testing.T) {
	c, err := loader.Load("testdata/dependent-chart-alias")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	if len(c.Dependencies()) == 0 {
		t.Fatal("There are no dependencies to run this test")
	}

	origLength := len(c.Dependencies())
	if err := ProcessRequirementsEnabled(c, c.Values); err != nil {
		t.Fatalf("Expected no errors but got %q", err)
	}

	if len(c.Dependencies()) == origLength {
		t.Fatal("Expected alias dependencies to be added, but did not got that")
	}

	if len(c.Dependencies()) != len(c.Requirements.Dependencies) {
		t.Fatalf("Expected number of chart dependencies %d, but got %d", len(c.Requirements.Dependencies), len(c.Dependencies()))
	}
}

func TestDependentChartWithSubChartsAbsentInRequirements(t *testing.T) {
	c, err := loader.Load("testdata/dependent-chart-no-requirements-yaml")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	if len(c.Dependencies()) != 2 {
		t.Fatalf("Expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	}

	origLength := len(c.Dependencies())
	if err := ProcessRequirementsEnabled(c, c.Values); err != nil {
		t.Fatalf("Expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != origLength {
		t.Fatal("Expected no changes in dependencies to be, but did something got changed")
	}
}

func TestDependentChartWithSubChartsHelmignore(t *testing.T) {
	if _, err := loader.Load("testdata/dependent-chart-helmignore"); err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
}

func TestDependentChartsWithSubChartsSymlink(t *testing.T) {
	c, err := loader.Load("testdata/joonix")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	if c.Name() != "joonix" {
		t.Fatalf("Unexpected chart name: %s", c.Name())
	}
	if n := len(c.Dependencies()); n != 1 {
		t.Fatalf("Expected 1 dependency for this chart, but got %d", n)
	}
}

func TestDependentChartsWithSubchartsAllSpecifiedInRequirements(t *testing.T) {
	c, err := loader.Load("testdata/dependent-chart-with-all-in-requirements-yaml")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	if len(c.Dependencies()) == 0 {
		t.Fatal("There are no dependencies to run this test")
	}

	origLength := len(c.Dependencies())
	if err := ProcessRequirementsEnabled(c, c.Values); err != nil {
		t.Fatalf("Expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != origLength {
		t.Fatal("Expected no changes in dependencies to be, but did something got changed")
	}

	if len(c.Dependencies()) != len(c.Requirements.Dependencies) {
		t.Fatalf("Expected number of chart dependencies %d, but got %d", len(c.Requirements.Dependencies), len(c.Dependencies()))
	}
}

func TestDependentChartsWithSomeSubchartsSpecifiedInRequirements(t *testing.T) {
	c, err := loader.Load("testdata/dependent-chart-with-mixed-requirements-yaml")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	if len(c.Dependencies()) == 0 {
		t.Fatal("There are no dependencies to run this test")
	}

	origLength := len(c.Dependencies())
	if err := ProcessRequirementsEnabled(c, c.Values); err != nil {
		t.Fatalf("Expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != origLength {
		t.Fatal("Expected no changes in dependencies to be, but did something got changed")
	}

	if len(c.Dependencies()) <= len(c.Requirements.Dependencies) {
		t.Fatalf("Expected more dependencies than specified in requirements.yaml(%d), but got %d", len(c.Requirements.Dependencies), len(c.Dependencies()))
	}
}

func verifyRequirements(t *testing.T, c *chart.Chart) {
	if len(c.Requirements.Dependencies) != 2 {
		t.Errorf("Expected 2 requirements, got %d", len(c.Requirements.Dependencies))
	}
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := c.Requirements.Dependencies[i]
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
	if len(c.Requirements.Dependencies) != 2 {
		t.Errorf("Expected 2 requirements, got %d", len(c.Requirements.Dependencies))
	}
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := c.Requirements.Dependencies[i]
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
