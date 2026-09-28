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
package util

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

func loadChart(t *testing.T, path string) *chart.Chart {
	t.Helper()
	c, err := loader.Load(path)
	require.NoError(t, err, "failed to load testdata")
	return c
}

func TestLoadDependency(t *testing.T) {
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}

	check := func(deps []*chart.Dependency) {
		require.Len(t, deps, 2, "expected 2 dependencies, got %d", len(deps))
		for i, tt := range tests {
			assert.Equal(t, tt.Name, deps[i].Name, "expected dependency named %q, got %q", tt.Name, deps[i].Name)
			assert.Equal(t, tt.Version, deps[i].Version, "expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, deps[i].Version)
			assert.Equal(t, tt.Repository, deps[i].Repository, "expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, deps[i].Repository)
		}
	}
	c := loadChart(t, "testdata/frobnitz")
	check(c.Metadata.Dependencies)
	check(c.Lock.Dependencies)
}

func TestDependencyEnabled(t *testing.T) {
	type M = map[string]any
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
			require.NoErrorf(t, processDependencyEnabled(c, tc.v, ""), "error processing enabled dependencies")

			names := extractChartNames(c)
			require.Len(t, names, len(tc.e), "slice lengths do not match got %v, expected %v", len(names), len(tc.e))
			for i := range names {
				require.Equal(t, tc.e[i], names[i], "slice values do not match got %v, expected %v", names, tc.e)
			}
		})
	}
}

// extractChartNames recursively searches chart dependencies returning all charts found
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

	// These values are imported from the child chart to the parent. Parent
	// values take precedence over imported values. This enables importing a
	// large section from a child chart and overriding a selection from it.
	e["overridden-chart1.SC1bool"] = "false"
	e["overridden-chart1.SC1float"] = "3.141592"
	e["overridden-chart1.SC1int"] = "99"
	e["overridden-chart1.SC1string"] = "pollywog"
	e["overridden-chart1.SPextra2"] = "42"

	e["overridden-chartA.SCAbool"] = "true"
	e["overridden-chartA.SCAfloat"] = "41.3"
	e["overridden-chartA.SCAint"] = "808"
	e["overridden-chartA.SCAstring"] = "jabberwocky"
	e["overridden-chartA.SPextra4"] = "true"

	// These values are imported from the child chart to the parent. Parent
	// values take precedence over imported values. This enables importing a
	// large section from a child chart and overriding a selection from it.
	e["overridden-chartA-B.SCAbool"] = "true"
	e["overridden-chartA-B.SCAfloat"] = "41.3"
	e["overridden-chartA-B.SCAint"] = "808"
	e["overridden-chartA-B.SCAstring"] = "jabberwocky"
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

	require.NoErrorf(t, processDependencyImportValues(c, false), "processing import values dependencies")
	cc := common.Values(c.Values)
	for kk, vv := range e {
		pv, err := cc.PathValue(kk)
		require.NoError(t, err, "retrieving import values table %v", kk)

		switch pv := pv.(type) {
		case float64:
			s := strconv.FormatFloat(pv, 'f', -1, 64)
			assert.Equalf(t, s, vv, "failed to match imported float value %v with expected %v for key %q", s, vv, kk)
		case bool:
			b := strconv.FormatBool(pv)
			assert.Equalf(t, b, vv, "failed to match imported bool value %v with expected %v for key %q", b, vv, kk)
		default:
			assert.Equal(t, vv, pv, "failed to match imported string value %q with expected %q for key %q", pv, vv, kk)
		}
	}

	// Since this was processed with coalescing there should be no null values.
	// Here we verify that.
	_, err := cc.PathValue("ensurenull")
	require.Error(t, err, "expect nil value not found but found it")
	var xerr common.ErrNoValue
	require.ErrorAs(t, err, &xerr, "expected an ErrNoValue")

	c = loadChart(t, "testdata/subpop")
	require.NoErrorf(t, processDependencyImportValues(c, true), "processing import values dependencies")
	cc = common.Values(c.Values)
	val, err := cc.PathValue("ensurenull")
	require.NoError(t, err, "expect value but ensurenull was not found")
	assert.Nil(t, val, "expect nil value but got %q instead", val)
}

func TestProcessDependencyImportValuesFromSharedDependencyToAliases(t *testing.T) {
	c := loadChart(t, "testdata/chart-with-import-from-aliased-dependencies")

	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.NoErrorf(t, processDependencyImportValues(c, true), "processing import values dependencies")
	e := make(map[string]string)

	e["foo-defaults.defaultValue"] = "42"
	e["bar-defaults.defaultValue"] = "42"

	e["foo.defaults.defaultValue"] = "42"
	e["bar.defaults.defaultValue"] = "42"

	e["foo.grandchild.defaults.defaultValue"] = "42"
	e["bar.grandchild.defaults.defaultValue"] = "42"

	cValues := common.Values(c.Values)
	for kk, vv := range e {
		pv, err := cValues.PathValue(kk)
		require.NoError(t, err, "retrieving import values table %v", kk)
		assert.Equal(t, vv, pv, "failed to match imported value %v with expected %v", pv, vv)
	}
}

func TestProcessDependencyImportValuesMultiLevelPrecedence(t *testing.T) {
	c := loadChart(t, "testdata/three-level-dependent-chart/umbrella")

	e := make(map[string]string)

	// The order of precedence should be:
	// 1. User specified values (e.g CLI)
	// 2. Parent chart values
	// 3. Imported values
	// 4. Sub-chart values
	// The 4 app charts here deal with things differently:
	// - app1 has a port value set in the umbrella chart. It does not import any
	//   values so the value from the umbrella chart should be used.
	// - app2 has a value in the app chart and imports from the library. The
	//   app chart value should take precedence.
	// - app3 has no value in the app chart and imports the value from the library
	//   chart. The library chart value should be used.
	// - app4 has a value in the app chart and does not import the value from the
	//   library chart. The app charts value should be used.
	e["app1.service.port"] = "3456"
	e["app2.service.port"] = "8080"
	e["app3.service.port"] = "9090"
	e["app4.service.port"] = "1234"
	require.NoErrorf(t, processDependencyImportValues(c, true), "processing import values dependencies")
	cc := common.Values(c.Values)
	for kk, vv := range e {
		pv, err := cc.PathValue(kk)
		require.NoError(t, err, "retrieving import values table %v", kk)

		switch pv := pv.(type) {
		case float64:
			s := strconv.FormatFloat(pv, 'f', -1, 64)
			assert.Equalf(t, s, vv, "failed to match imported float value %v with expected %v", s, vv)
		default:
			assert.Equal(t, vv, pv, "failed to match imported string value %q with expected %q", pv, vv)
		}
	}
}

func TestProcessDependencyImportValuesForEnabledCharts(t *testing.T) {
	c := loadChart(t, "testdata/import-values-from-enabled-subchart/parent-chart")
	nameOverride := "parent-chart-prod"

	require.NoErrorf(t, processDependencyImportValues(c, true), "processing import values dependencies")
	require.Len(t, c.Dependencies(), 2, "expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.Len(t, c.Dependencies(), 1, "expected no changes in dependencies")
	require.Len(t, c.Metadata.Dependencies, 1, "expected 1 dependency specified in Chart.yaml, got %d", len(c.Metadata.Dependencies))
	prodDependencyValues := c.Dependencies()[0].Values
	require.Equal(t, nameOverride, prodDependencyValues["nameOverride"], "dependency chart name should be %s but got %s", nameOverride, prodDependencyValues["nameOverride"])
}

func TestGetAliasDependency(t *testing.T) {
	c := loadChart(t, "testdata/frobnitz")
	req := c.Metadata.Dependencies

	require.NotEmpty(t, req, "there are no dependencies to test")

	// Success case
	aliasChart := getAliasDependency(c.Dependencies(), req[0])
	require.NotNil(t, aliasChart, "failed to get dependency chart for alias %s", req[0].Name)
	if req[0].Alias != "" {
		require.Equal(t, req[0].Alias, aliasChart.Name(), "dependency chart name should be %s but got %s", req[0].Alias, aliasChart.Name())
	} else {
		require.Equalf(t, aliasChart.Name(), req[0].Name, "dependency chart name should be %s but got %s", req[0].Name, aliasChart.Name())
	}

	if req[0].Version != "" {
		require.True(t, IsCompatibleRange(req[0].Version, aliasChart.Metadata.Version), "dependency chart version is not in the compatible range")
	}

	// Failure case
	req[0].Name = "something-else"
	require.Nilf(t, getAliasDependency(c.Dependencies(), req[0]), "expected no chart")

	req[0].Version = "something else which is not in the compatible range"
	require.False(t, IsCompatibleRange(req[0].Version, aliasChart.Metadata.Version), "dependency chart version outside the compatible range should fail, but it succeeded")
}

func TestDependentChartAliases(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-alias")
	req := c.Metadata.Dependencies

	require.Len(t, c.Dependencies(), 2, "expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.Len(t, c.Dependencies(), 3, "expected alias dependencies to be added")
	require.Len(t, c.Dependencies(), len(c.Metadata.Dependencies), "expected number of chart dependencies %d, but got %d", len(c.Metadata.Dependencies), len(c.Dependencies()))

	aliasChart := getAliasDependency(c.Dependencies(), req[2])

	require.NotNil(t, aliasChart, "failed to get dependency chart for alias %s", req[2].Name)
	require.Equal(t, c, aliasChart.Parent(), "dependency chart has wrong parent, expected %s but got %s", c.Name(), aliasChart.Parent().Name())
	if req[2].Alias != "" {
		require.Equal(t, req[2].Alias, aliasChart.Name(), "dependency chart name should be %s but got %s", req[2].Alias, aliasChart.Name())
	} else {
		require.Equalf(t, aliasChart.Name(), req[2].Name, "dependency chart name should be %s but got %s", req[2].Name, aliasChart.Name())
	}

	req[2].Name = "dummy-name"
	require.Nilf(t, getAliasDependency(c.Dependencies(), req[2]), "expected no chart")
}

func TestDependentChartWithSubChartsAbsentInDependency(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-no-requirements-yaml")

	require.Len(t, c.Dependencies(), 2, "expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.Len(t, c.Dependencies(), 2, "expected no changes in dependencies")
}

func TestDependentChartWithSubChartsHelmignore(t *testing.T) {
	// FIXME what does this test?
	loadChart(t, "testdata/dependent-chart-helmignore")
}

func TestDependentChartsWithSubChartsSymlink(t *testing.T) {
	joonix := filepath.Join("testdata", "joonix")
	require.NoError(t, os.Symlink(filepath.Join("..", "..", "frobnitz"), filepath.Join(joonix, "charts", "frobnitz")))
	defer os.RemoveAll(filepath.Join(joonix, "charts", "frobnitz"))
	c := loadChart(t, joonix)

	require.Equal(t, "joonix", c.Name(), "unexpected chart name: %s", c.Name())
	require.Lenf(t, c.Dependencies(), 1, "expected 1 dependency for this chart")
}

func TestDependentChartsWithSubchartsAllSpecifiedInDependency(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-with-all-in-requirements-yaml")

	require.Len(t, c.Dependencies(), 2, "expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.Len(t, c.Dependencies(), 2, "expected no changes in dependencies")
	require.Len(t, c.Dependencies(), len(c.Metadata.Dependencies), "expected number of chart dependencies %d, but got %d", len(c.Metadata.Dependencies), len(c.Dependencies()))
}

func TestDependentChartsWithSomeSubchartsSpecifiedInDependency(t *testing.T) {
	c := loadChart(t, "testdata/dependent-chart-with-mixed-requirements-yaml")

	require.Len(t, c.Dependencies(), 2, "expected 2 dependencies for this chart, but got %d", len(c.Dependencies()))
	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.Len(t, c.Dependencies(), 2, "expected no changes in dependencies")
	require.Len(t, c.Metadata.Dependencies, 1, "expected 1 dependency specified in Chart.yaml, got %d", len(c.Metadata.Dependencies))
}

func validateDependencyTree(t *testing.T, c *chart.Chart) {
	t.Helper()
	for _, dependency := range c.Dependencies() {
		if dependency.Parent() != c {
			require.Equal(t, c, dependency.Parent(), "dependency chart %s has wrong parent, expected %s but got %s", dependency.Name(), c.Name(), dependency.Parent().Name())
		}
		// recurse entire tree
		validateDependencyTree(t, dependency)
	}
}

func TestChartWithDependencyAliasedTwiceAndDoublyReferencedSubDependency(t *testing.T) {
	c := loadChart(t, "testdata/chart-with-dependency-aliased-twice")

	require.Len(t, c.Dependencies(), 1, "expected one dependency for this chart, but got %d", len(c.Dependencies()))
	require.NoErrorf(t, processDependencyEnabled(c, c.Values, ""), "expected no errors")
	require.Len(t, c.Dependencies(), 2, "expected two dependencies after processing aliases")

	validateDependencyTree(t, c)
}
