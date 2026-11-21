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

package cmd

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	"helm.sh/helm/v4/internal/gates"
	"helm.sh/helm/v4/internal/test"
	"helm.sh/helm/v4/pkg/chart/common"
)

func TestDagCmd(t *testing.T) {
	t.Setenv(string(gates.ChartV3), "1")
	for _, tt := range []struct {
		name, golden, path string
		chart              *chart.Chart
	}{
		{name: "linear chain", golden: "output/dag-v3-linear.txt", chart: linearDagChart()},
		{name: "diamond with alias and parent annotation", golden: "output/dag-v3-diamond.txt", chart: diamondDagChart()},
		{name: "three nested levels", golden: "output/dag-v3-nested.txt", chart: nestedDagChart()},
		{
			name:   "resource groups compose with subchart order",
			golden: "output/dag-v3-sequenced-groups.txt",
			path:   "testdata/testcharts/v3-sequenced-groups",
		},
		{
			name:   "isolated resource groups are deployed unsequenced",
			golden: "output/dag-v3-sequenced-isolated.txt",
			path:   "testdata/testcharts/v3-sequenced-isolated",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if tt.chart != nil {
				path = saveDagChart(t, tt.chart)
			}
			_, first, err := executeActionCommandC(storageFixture(), fmt.Sprintf("dag %q", path))
			require.NoError(t, err)
			_, second, err := executeActionCommandC(storageFixture(), fmt.Sprintf("dag %q", path))
			require.NoError(t, err)
			assert.Equal(t, first, second)
			test.AssertGoldenString(t, first, tt.golden)
		})
	}
}

func TestDagCmd_Errors(t *testing.T) {
	t.Setenv(string(gates.ChartV3), "1")
	cycle := dagChart("cycle")
	setDagDependencies(cycle,
		dagDependency(dagChart("alpha"), "beta"),
		dagDependency(dagChart("beta"), "alpha"),
	)
	unknown := dagChart("unknown")
	setDagDependencies(unknown, dagDependency(dagChart("child"), "missing"))

	for _, tt := range []struct {
		name, command string
		contains      []string
	}{
		{"cycle", fmt.Sprintf("dag %q", saveDagChart(t, cycle)), []string{"cycle", "alpha", "beta"}},
		{"unknown dependency", fmt.Sprintf("dag %q", saveDagChart(t, unknown)), []string{"unknown or disabled subchart", "missing"}},
		{"non-v3 chart", "dag testdata/testcharts/alpine", []string{"apiVersion", "v1", "v3"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := executeActionCommandC(storageFixture(), tt.command)
			require.Error(t, err)
			for _, want := range tt.contains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestDagCmd_RequiresChartArg(t *testing.T) {
	t.Setenv(string(gates.ChartV3), "1")
	_, _, err := executeActionCommandC(storageFixture(), "dag")
	require.ErrorContains(t, err, "requires 1 argument")
}

func TestDagCmd_NonexistentChart(t *testing.T) {
	t.Setenv(string(gates.ChartV3), "1")
	_, _, err := executeActionCommandC(storageFixture(), "dag testdata/testcharts/does-not-exist")
	require.ErrorContains(t, err, "does-not-exist")
}

func TestDagCmd_GateDisabled(t *testing.T) {
	t.Setenv(string(gates.ChartV3), "")
	_, _, err := executeActionCommandC(storageFixture(), "dag missing")
	require.EqualError(t, err, gates.ChartV3.Error().Error())
}

func linearDagChart() *chart.Chart {
	root := dagChart("linear")
	setDagDependencies(root,
		dagDependency(dagChart("database")),
		dagDependency(dagChart("api"), "database"),
		dagDependency(dagChart("web"), "api"),
	)
	return root
}

func diamondDagChart() *chart.Chart {
	root := dagChart("diamond")
	root.Metadata.Annotations = map[string]string{chartutil.AnnotationDependsOnSubcharts: `["frontend"]`}
	database := dagDependency(dagChart("database"))
	database.metadata.Alias = "db"
	setDagDependencies(root, database,
		dagDependency(dagChart("api"), "database"),
		dagDependency(dagChart("cache"), "db"),
		dagDependency(dagChart("frontend"), "api", "cache"),
	)
	return root
}

func nestedDagChart() *chart.Chart {
	root, middle := dagChart("nested"), dagChart("middle")
	setDagDependencies(middle, dagDependency(dagChart("leaf")))
	setDagDependencies(root, dagDependency(middle))
	return root
}

type dagDependencyFixture struct {
	chart    *chart.Chart
	metadata *chart.Dependency
}

func dagDependency(chrt *chart.Chart, dependsOn ...string) dagDependencyFixture {
	return dagDependencyFixture{chrt, &chart.Dependency{Name: chrt.Name(), Version: chrt.Metadata.Version, DependsOn: dependsOn}}
}

func setDagDependencies(parent *chart.Chart, dependencies ...dagDependencyFixture) {
	charts := make([]*chart.Chart, 0, len(dependencies))
	for _, dependency := range dependencies {
		parent.Metadata.Dependencies = append(parent.Metadata.Dependencies, dependency.metadata)
		charts = append(charts, dependency.chart)
	}
	parent.SetDependencies(charts...)
}

func dagChart(name string) *chart.Chart {
	manifest := fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n", name)
	return &chart.Chart{
		Metadata:  &chart.Metadata{Name: name, Version: "0.1.0", APIVersion: chart.APIVersionV3},
		Templates: []*common.File{{Name: "templates/configmap.yaml", Data: []byte(manifest)}},
	}
}

func saveDagChart(t *testing.T, chrt *chart.Chart) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, chartutil.SaveDir(chrt, dir))
	return filepath.Join(dir, chrt.Name())
}
