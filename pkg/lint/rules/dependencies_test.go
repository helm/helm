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
	"testing"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/lint/support"
)

const (
	badChartDepsDir = "testdata/lint-deps"
)

func TestValidateDependencyInChartsDir(t *testing.T) {
	c, err := loader.Load(badChartDepsDir)
	if err != nil {
		t.Fatal(err)
	}

	if err = validateDependencyInChartsDir(c); err == nil {
		t.Errorf("chart %q should have been flagged for missing deps in chart directory", badChartDepsDir)
	}
}

func TestValidateDependencyInMetadata(t *testing.T) {
	c, err := loader.Load(badChartDepsDir)
	if err != nil {
		t.Fatal(err)
	}

	if err = validateDependencyInMetadata(c); err == nil {
		t.Errorf("chart %q should have been flagged for missing deps in chart metadata", badChartDepsDir)
	}
}

func TestDependencies(t *testing.T) {
	linter := support.Linter{ChartDir: badChartDepsDir}
	Dependencies(&linter)
	if l := len(linter.Messages); l != 2 {
		t.Errorf("expected 2 linter errors for bad chart dependencies. Got %d.", l)
	}
}
