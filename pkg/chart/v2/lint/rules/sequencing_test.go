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
	"path/filepath"
	"strings"
	"testing"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func TestSequencing_SubchartCircularDep(t *testing.T) {
	tmp := t.TempDir()
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "testchart",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{
				{Name: "subchart-a", DependsOn: []string{"subchart-b"}},
				{Name: "subchart-b", DependsOn: []string{"subchart-a"}},
			},
		},
	}
	if err := chartutil.SaveDir(c, tmp); err != nil {
		t.Fatalf("SaveDir: %v", err)
	}

	linter := support.Linter{ChartDir: filepath.Join(tmp, "testchart")}
	Sequencing(&linter, "testns", nil)

	// Expect at least one ErrorSev message about circular dependency
	found := false
	for _, msg := range linter.Messages {
		if msg.Severity == support.ErrorSev && strings.Contains(msg.Err.Error(), "circular") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected circular dependency error, got messages: %v", linter.Messages)
	}
}

func TestSequencing_SubchartNoDeps(t *testing.T) {
	tmp := t.TempDir()
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "testchart",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{
				{Name: "subchart-a"},
				{Name: "subchart-b"},
			},
		},
	}
	if err := chartutil.SaveDir(c, tmp); err != nil {
		t.Fatalf("SaveDir: %v", err)
	}

	linter := support.Linter{ChartDir: filepath.Join(tmp, "testchart")}
	Sequencing(&linter, "testns", nil)

	// No circular dependency errors expected
	for _, msg := range linter.Messages {
		if msg.Severity == support.ErrorSev && strings.Contains(msg.Err.Error(), "circular") {
			t.Errorf("unexpected circular dependency error: %v", msg)
		}
	}
}

func TestSequencing_PartialReadinessAnnotation(t *testing.T) {
	linter := support.Linter{ChartDir: "./testdata/sequencing-partial-readiness"}
	Sequencing(&linter, "testns", nil)

	// Expect an error about partial readiness annotation
	found := false
	for _, msg := range linter.Messages {
		if msg.Severity == support.ErrorSev && strings.Contains(msg.Err.Error(), "readiness") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected partial readiness annotation error, got messages: %v", linter.Messages)
	}
}

func TestSequencing_OrphanResourceGroup(t *testing.T) {
	linter := support.Linter{ChartDir: "./testdata/sequencing-orphan-group"}
	Sequencing(&linter, "testns", nil)

	// Expect a warning about non-existent group reference
	found := false
	for _, msg := range linter.Messages {
		if msg.Severity == support.WarningSev {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about non-existent group reference, got messages: %v", linter.Messages)
	}
}

func TestSequencing_ValidChart(t *testing.T) {
	linter := support.Linter{ChartDir: "./testdata/albatross"}
	Sequencing(&linter, "testns", map[string]interface{}{"nameOverride": "", "httpPort": 80})

	// Should produce no ErrorSev messages for sequencing issues
	for _, msg := range linter.Messages {
		if msg.Severity == support.ErrorSev {
			t.Errorf("unexpected error on valid chart: %v", msg)
		}
	}
}
