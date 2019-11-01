/*
Copyright The Helm Authors.
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
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func loadChart(t *testing.T, path string) *chart.Chart {
	t.Helper()
	c, err := loader.Load(path)
	if err != nil {
		t.Fatalf("failed to load testdata: %s", err)
	}
	return c
}

func TestLoadDependency(t *testing.T) {
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}

	check := func(deps []*chart.Dependency) {
		if len(deps) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(deps))
		}
		for i, tt := range tests {
			if deps[i].Name != tt.Name {
				t.Errorf("expected dependency named %q, got %q", tt.Name, deps[i].Name)
			}
			if deps[i].Version != tt.Version {
				t.Errorf("expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, deps[i].Version)
			}
			if deps[i].Repository != tt.Repository {
				t.Errorf("expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, deps[i].Repository)
			}
		}
	}
	c := loadChart(t, "testdata/frobnitz")
	check(c.Metadata.Dependencies)
	check(c.Lock.Dependencies)
}

// recProcessDependencyEnabled is mostly a simplified version of
// Engine.recUpdateRenderValues, for testing only dependencies
func recProcessDependencyEnabled(c *chart.Chart, v map[string]interface{}, tags map[string]interface{}) error {
	// get the local values
	var err error
	if c.IsRoot() {
		v, err = CoalesceRoot(c, v)
		tags = GetTags(v)
	} else {
		v, err = CoalesceDep(c, v)
	}
	if err != nil {
		return err
	}
	// Remove all disabled dependencies
	err = ProcessDependencyEnabled(c, v, tags)
	if err != nil {
		return err
	}
	// Recursive upudate on enabled dependencies
	for _, child := range c.Dependencies() {
		err = recProcessDependencyEnabled(child, v, tags)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestDependencyEnabled(t *testing.T) {
	type M = map[string]interface{}
	tests := []struct {
		name string
		v    M
		e    []string // expected charts including duplicates in alphanumeric order
	}{{
		"tags with no effect",
		M{"tags": M{"nothinguseful": false}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb"},
	}, {
		"tags disabling a group",
		M{"tags": M{"front-end": false}},
		[]string{"parentchart"},
	}, {
		"tags disabling a group and enabling a different group",
		M{"tags": M{"front-end": false, "back-end": true}},
		[]string{"parentchart", "parentchart.subchart2", "parentchart.subchart2.subchartb", "parentchart.subchart2.subchartc"},
	}, {
		"tags disabling only children, children still enabled since tag front-end=true in values.yaml",
		M{"tags": M{"subcharta": false, "subchartb": false}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb"},
	}, {
		"tags disabling all parents/children with additional tag re-enabling a parent",
		M{"tags": M{"front-end": false, "subchart1": true, "back-end": false}},
		[]string{"parentchart", "parentchart.subchart1"},
	}, {
		"conditions enabling the parent charts, but back-end (b, c) is still disabled via values.yaml",
		M{"subchart1": M{"enabled": true}, "subchart2": M{"enabled": true}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb", "parentchart.subchart2"},
	}, {
		"conditions disabling the parent charts, effectively disabling children",
		M{"subchart1": M{"enabled": false}, "subchart2": M{"enabled": false}},
		[]string{"parentchart"},
	}, {
		"conditions a child using the second condition path of child's condition",
		M{"subchart1": M{"subcharta": M{"enabled": false}}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subchartb"},
	}, {
		"tags enabling a parent/child group with condition disabling one child",
		M{"subchart2": M{"subchartc": M{"enabled": false}}, "tags": M{"back-end": true}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb", "parentchart.subchart2", "parentchart.subchart2.subchartb"},
	}, {
		"tags will not enable a child if parent is explicitly disabled with condition",
		M{"subchart1": M{"enabled": false}, "tags": M{"front-end": true}},
		[]string{"parentchart"},
	}, {
		"subcharts with alias also respect conditions",
		M{"subchart1": M{"enabled": false}, "subchart2alias": M{"enabled": true, "subchartb": M{"enabled": true}}},
		[]string{"parentchart", "parentchart.subchart2alias", "parentchart.subchart2alias.subchartb"},
	}}

	for _, tc := range tests {
		c := loadChart(t, "testdata/subpop")
		t.Run(tc.name, func(t *testing.T) {
			if err := recProcessDependencyEnabled(c, tc.v, nil); err != nil {
				t.Fatalf("error processing enabled dependencies %v", err)
			}

			names := extractChartNames(c)
			if len(names) != len(tc.e) {
				t.Fatalf("slice lengths do not match got %v, expected %v", len(names), len(tc.e))
			}
			for i := range names {
				if names[i] != tc.e[i] {
					t.Fatalf("slice values do not match got %v, expected %v", names, tc.e)
				}
			}
		})
	}
}

// extractCharts recursively searches chart dependencies returning all charts found
func extractChartNames(c *chart.Chart) []string {
	var out []string
	var fn func(c *chart.Chart)
	fn = func(c *chart.Chart) {
		out = append(out, c.ChartPath())
		for _, d := range c.Dependencies() {
			fn(d)
		}
	}
	fn(c)
	sort.Strings(out)
	return out
}

func TestProcessDependencyImportValues(t *testing.T) {
	c := loadChart(t, "testdata/subpop")

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

	cvals, err := CoalesceValues(c, nil)
	if err != nil {
		t.Fatalf("coalescing values %v", err)
	}
	if err := ProcessDependencyImportValues(c, cvals); err != nil {
		t.Fatalf("processing import values dependencies %v", err)
	}
	cc := Values(cvals)
	for kk, vv := range e {
		pv, err := cc.PathValue(kk)
		if err != nil {
			t.Fatalf("retrieving import values table %v %v", kk, err)
		}

		switch pv := pv.(type) {
		case float64:
			if s := strconv.FormatFloat(pv, 'f', -1, 64); s != vv {
				t.Errorf("failed to match imported float value %v with expected %v", s, vv)
			}
		case bool:
			if b := strconv.FormatBool(pv); b != vv {
				t.Errorf("failed to match imported bool value %v with expected %v", b, vv)
			}
		default:
			if pv != vv {
				t.Errorf("failed to match imported string value %q with expected %q", pv, vv)
			}
		}
	}
}

func TestGetAliasDependency(t *testing.T) {
	c := loadChart(t, "testdata/frobnitz")
	req := c.Metadata.Dependencies

	if len(req) == 0 {
		t.Fatalf("there are no dependencies to test")
	}

	// Success case
	aliasChart := getAliasDependency(c.Dependencies(), req[0])
	if aliasChart == nil {
		t.Fatalf("failed to get dependency chart for alias %s", req[0].Name)
	}
	if req[0].Alias != "" {
		if aliasChart.Name() != req[0].Alias {
			t.Fatalf("dependency chart name should be %s but got %s", req[0].Alias, aliasChart.Name())
		}
	} else if aliasChart.Name() != req[0].Name {
		t.Fatalf("dependency chart name should be %s but got %s", req[0].Name, aliasChart.Name())
	}

	if req[0].Version != "" {
		if !IsCompatibleRange(req[0].Version, aliasChart.Metadata.Version) {
			t.Fatalf("dependency chart version is not in the compatible range")
		}
	}

	// Failure case
	req[0].Name = "something-else"
	if aliasChart := getAliasDependency(c.Dependencies(), req[0]); aliasChart != nil {
		t.Fatalf("expected no chart but got %s", aliasChart.Name())
	}

	req[0].Version = "something else which is not in the compatible range"
	if IsCompatibleRange(req[0].Version, aliasChart.Metadata.Version) {
		t.Fatalf("dependency chart version which is not in the compatible range should cause a failure other than a success ")
	}
}

func TestDependentChartAliases(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-alias")

	if len(c.Dependencies()) != 2 {
		t.Fatalf("expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	}

	if err := ProcessDependencyEnabled(c, c.Values, GetTags(c.Values)); err != nil {
		t.Fatalf("expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != 3 {
		t.Fatal("expected alias dependencies to be added")
	}

	if len(c.Dependencies()) != len(c.Metadata.Dependencies) {
		t.Fatalf("expected number of chart dependencies %d, but got %d", len(c.Metadata.Dependencies), len(c.Dependencies()))
	}
	// FIXME test for correct aliases
}

func TestDependentChartWithSubChartsAbsentInDependency(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-no-requirements-yaml")

	if len(c.Dependencies()) != 2 {
		t.Fatalf("expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	}

	if err := ProcessDependencyEnabled(c, c.Values, GetTags(c.Values)); err != nil {
		t.Fatalf("expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != 2 {
		t.Fatal("expected no changes in dependencies")
	}
}

func TestDependentChartWithSubChartsHelmignore(t *testing.T) {
	// FIXME what does this test?
	loadChart(t, "testdata/dependent-chart-helmignore")
}

func TestDependentChartsWithSubChartsSymlink(t *testing.T) {
	joonix := filepath.Join("testdata", "joonix")
	if err := os.Symlink(filepath.Join("..", "..", "frobnitz"), filepath.Join(joonix, "charts", "frobnitz")); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Join(joonix, "charts", "frobnitz"))
	c := loadChart(t, joonix)

	if c.Name() != "joonix" {
		t.Fatalf("unexpected chart name: %s", c.Name())
	}
	if n := len(c.Dependencies()); n != 1 {
		t.Fatalf("expected 1 dependency for this chart, but got %d", n)
	}
}

func TestDependentChartsWithSubchartsAllSpecifiedInDependency(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-with-all-in-requirements-yaml")

	if len(c.Dependencies()) != 2 {
		t.Fatalf("expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	}

	if err := ProcessDependencyEnabled(c, c.Values, GetTags(c.Values)); err != nil {
		t.Fatalf("expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != 2 {
		t.Fatal("expected no changes in dependencies")
	}

	if len(c.Dependencies()) != len(c.Metadata.Dependencies) {
		t.Fatalf("expected number of chart dependencies %d, but got %d", len(c.Metadata.Dependencies), len(c.Dependencies()))
	}
}

func TestDependentChartsWithSomeSubchartsSpecifiedInDependency(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-with-mixed-requirements-yaml")

	if len(c.Dependencies()) != 2 {
		t.Fatalf("expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	}

	if err := ProcessDependencyEnabled(c, c.Values, GetTags(c.Values)); err != nil {
		t.Fatalf("expected no errors but got %q", err)
	}

	if len(c.Dependencies()) != 2 {
		t.Fatal("expected no changes in dependencies")
	}

	if len(c.Metadata.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency specified in Chart.yaml, got %d", len(c.Metadata.Dependencies))
	}
}

func TestGetTags(t *testing.T) {
	type M = map[string]interface{}
	tests := []struct {
		name string
		vals M
		tags M
	}{{
		"normal tags",
		M{"tags": M{"a": true, "b": false}},
		M{"a": true, "b": false},
	}, {
		"not an object tags",
		M{"tags": []interface{}{"a", "b"}},
		nil,
	}, {
		"no tags",
		M{"no_tags": M{"a": true, "b": false}},
		nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := GetTags(tt.vals)
			if !reflect.DeepEqual(tags, tt.tags) {
				t.Fatalf("tags map do not match got %v, expected %v", tags, tt.tags)
			}
		})
	}
}
