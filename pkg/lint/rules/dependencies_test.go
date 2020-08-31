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
package rules

import (
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
)

func chartWithBadDependencies() chart.Chart {
	badChartDeps := chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "badchart",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{
				{
					Name: "sub2",
				},
				{
					Name: "sub3",
				},
			},
		},
	}

	badChartDeps.SetDependencies(
		&chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "sub1",
				Version:    "0.1.0",
				APIVersion: "v2",
			},
		},
		&chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "sub2",
				Version:    "0.1.0",
				APIVersion: "v2",
			},
		},
	)
	return badChartDeps
}

func TestValidateDependencyInChartsDir(t *testing.T) {
	c := chartWithBadDependencies()

	if err := validateDependencyInChartsDir(&c); err == nil {
		t.Error("chart should have been flagged for missing deps in chart directory")
	}
}

func TestValidateDependencyInMetadata(t *testing.T) {
	c := chartWithBadDependencies()

	if err := validateDependencyInMetadata(&c); err == nil {
		t.Errorf("chart should have been flagged for missing deps in chart metadata")
	}
}

func TestDependencies(t *testing.T) {
	tmp := ensure.TempDir(t)
	defer os.RemoveAll(tmp)

	c := chartWithBadDependencies()
	err := chartutil.SaveDir(&c, tmp)
	if err != nil {
		t.Fatal(err)
	}
	linter := support.Linter{ChartDir: filepath.Join(tmp, c.Metadata.Name)}

	Dependencies(&linter)
	if l := len(linter.Messages); l != 2 {
		t.Errorf("expected 2 linter errors for bad chart dependencies. Got %d.", l)
		for i, msg := range linter.Messages {
			t.Logf("Message: %d, Error: %#v", i, msg)
		}
	}
}
