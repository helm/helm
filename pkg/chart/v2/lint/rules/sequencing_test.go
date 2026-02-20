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

	"github.com/stretchr/testify/assert"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func TestValidateSubchartDependsOn(t *testing.T) {
	tests := []struct {
		name        string
		metadata    *chart.Metadata
		expectError bool
	}{
		{
			name: "valid depends-on",
			metadata: &chart.Metadata{
				Name: "parent",
				Dependencies: []*chart.Dependency{
					{Name: "redis"},
					{Name: "app", DependsOn: []string{"redis"}},
				},
			},
			expectError: false,
		},
		{
			name: "unknown depends-on reference",
			metadata: &chart.Metadata{
				Name: "parent",
				Dependencies: []*chart.Dependency{
					{Name: "app", DependsOn: []string{"ghost"}},
				},
			},
			expectError: true,
		},
		{
			name: "valid annotation reference",
			metadata: &chart.Metadata{
				Name: "parent",
				Annotations: map[string]string{
					chartutil.AnnotationDependsOnSubcharts: `["redis"]`,
				},
				Dependencies: []*chart.Dependency{
					{Name: "redis"},
				},
			},
			expectError: false,
		},
		{
			name: "unknown annotation reference",
			metadata: &chart.Metadata{
				Name: "parent",
				Annotations: map[string]string{
					chartutil.AnnotationDependsOnSubcharts: `["ghost"]`,
				},
				Dependencies: []*chart.Dependency{
					{Name: "redis"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubchartDependsOn(tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSubchartDAG(t *testing.T) {
	tests := []struct {
		name        string
		metadata    *chart.Metadata
		expectError bool
	}{
		{
			name: "no sequencing - no error",
			metadata: &chart.Metadata{
				Name: "parent",
				Dependencies: []*chart.Dependency{
					{Name: "redis"},
				},
			},
			expectError: false,
		},
		{
			name: "valid DAG",
			metadata: &chart.Metadata{
				Name: "parent",
				Dependencies: []*chart.Dependency{
					{Name: "redis"},
					{Name: "app", DependsOn: []string{"redis"}},
				},
			},
			expectError: false,
		},
		{
			name: "circular dependency",
			metadata: &chart.Metadata{
				Name: "parent",
				Dependencies: []*chart.Dependency{
					{Name: "A", DependsOn: []string{"B"}},
					{Name: "B", DependsOn: []string{"A"}},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubchartDAG(tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
