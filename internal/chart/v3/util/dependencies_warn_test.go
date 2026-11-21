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
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
)

// parentChartWithMissingImport returns a parent chart whose dependency
// "imported" imports a values table that does not exist. Processing
// import-values must therefore emit the "ImportValues missing table from
// chart" warning for that dependency.
func parentChartWithMissingImport() *chart.Chart {
	child := &chart.Chart{
		Metadata: &chart.Metadata{APIVersion: chart.APIVersionV3, Name: "imported", Version: "1.0.0"},
		Values:   map[string]any{"present": "value"},
	}
	parent := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV3,
			Name:       "parent",
			Version:    "1.0.0",
			Dependencies: []*chart.Dependency{{
				Name:       "imported",
				Version:    "1.0.0",
				Repository: "file://./charts/imported",
				ImportValues: []any{map[string]any{
					"child":  "absent",
					"parent": "importedValues",
				}},
			}},
		},
		Values: map[string]any{"imported": map[string]any{"present": "value"}},
	}
	parent.AddDependency(child)
	return parent
}

func TestProcessImportValuesMissingTableWarns(t *testing.T) {
	tests := []struct {
		name  string
		merge bool
	}{
		{name: "coalesce", merge: false},
		{name: "merge", merge: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			require.NoError(t, processImportValues(parentChartWithMissingImport(), tt.merge))

			logged := logBuf.String()
			require.Contains(t, logged, "msg=\"ImportValues missing table from chart\"",
				"expected a warning about the missing import-values table, got: %q", logged)
			// The warning must identify the dependency whose table is missing,
			// matching the v2 implementation (pkg/chart/v2/util/dependencies.go).
			assert.Contains(t, logged, "chart=imported",
				"warning must name the dependency in the chart attribute, got: %q", logged)
			assert.NotContains(t, logged, "chart=chart",
				"chart attribute must not be the literal string \"chart\", got: %q", logged)
		})
	}
}
