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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestParseDependsOnSubcharts(t *testing.T) {
	tests := []struct {
		name        string
		metadata    *chart.Metadata
		expected    []string
		expectError bool
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			expected: nil,
		},
		{
			name:     "no annotations",
			metadata: &chart.Metadata{},
			expected: nil,
		},
		{
			name: "no depends-on annotation",
			metadata: &chart.Metadata{
				Annotations: map[string]string{
					"other": "value",
				},
			},
			expected: nil,
		},
		{
			name: "valid annotation",
			metadata: &chart.Metadata{
				Annotations: map[string]string{
					AnnotationDependsOnSubcharts: `["bar", "rabbitmq"]`,
				},
			},
			expected: []string{"bar", "rabbitmq"},
		},
		{
			name: "empty array",
			metadata: &chart.Metadata{
				Annotations: map[string]string{
					AnnotationDependsOnSubcharts: `[]`,
				},
			},
			expected: []string{},
		},
		{
			name: "invalid JSON",
			metadata: &chart.Metadata{
				Annotations: map[string]string{
					AnnotationDependsOnSubcharts: `not-json`,
				},
			},
			expectError: true,
		},
		{
			name: "empty string value",
			metadata: &chart.Metadata{
				Annotations: map[string]string{
					AnnotationDependsOnSubcharts: "",
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDependsOnSubcharts(tt.metadata)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseResourceGroupAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		doc         string
		expected    *ResourceGroupAnnotation
		expectError bool
	}{
		{
			name: "no annotation",
			doc: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test`,
			expected: nil,
		},
		{
			name: "resource-group only",
			doc: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    helm.sh/resource-group: database`,
			expected: &ResourceGroupAnnotation{Group: "database"},
		},
		{
			name: "resource-group with depends-on",
			doc: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  annotations:
    helm.sh/resource-group: application
    helm.sh/depends-on/resource-groups: '["database", "cache"]'`,
			expected: &ResourceGroupAnnotation{
				Group:     "application",
				DependsOn: []string{"database", "cache"},
			},
		},
		{
			name: "invalid depends-on JSON",
			doc: `apiVersion: v1
kind: Service
metadata:
  annotations:
    helm.sh/resource-group: app
    helm.sh/depends-on/resource-groups: not-json`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseResourceGroupAnnotations(tt.doc)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBuildResourceGroupDAG(t *testing.T) {
	tests := []struct {
		name           string
		docs           []string
		expectedBatch  [][]string
		expectWarnings int
		expectError    bool
		errorContains  string
	}{
		{
			name:          "no resource groups",
			docs:          []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test"},
			expectedBatch: nil,
		},
		{
			name: "linear dependency: database -> application",
			docs: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: database",
				"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  annotations:\n    helm.sh/resource-group: application\n    helm.sh/depends-on/resource-groups: '[\"database\"]'",
			},
			expectedBatch: [][]string{{"database"}, {"application"}},
		},
		{
			name: "parallel groups with shared downstream",
			docs: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: database",
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: cache",
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: app\n    helm.sh/depends-on/resource-groups: '[\"database\", \"cache\"]'",
			},
			expectedBatch: [][]string{{"cache", "database"}, {"app"}},
		},
		{
			name: "warning on non-existent group reference",
			docs: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: app\n    helm.sh/depends-on/resource-groups: '[\"ghost\"]'",
			},
			expectedBatch:  [][]string{{"app"}},
			expectWarnings: 1,
		},
		{
			name: "cycle detection",
			docs: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: A\n    helm.sh/depends-on/resource-groups: '[\"B\"]'",
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    helm.sh/resource-group: B\n    helm.sh/depends-on/resource-groups: '[\"A\"]'",
			},
			expectError:   true,
			errorContains: "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag, _, _, warnings, err := BuildResourceGroupDAG(tt.docs)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.expectedBatch == nil {
					assert.Equal(t, 0, dag.Len())
				} else {
					batches, err := dag.Batches()
					require.NoError(t, err)
					assert.Equal(t, tt.expectedBatch, batches)
				}
				assert.Len(t, warnings, tt.expectWarnings)
			}
		})
	}
}

func TestBuildSubchartDAG(t *testing.T) {
	tests := []struct {
		name           string
		chart          *chart.Chart
		expectedBatch  [][]string
		expectError    bool
		errorContains  string
	}{
		{
			name: "no dependencies",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name: "myapp",
				},
			},
			expectedBatch: [][]string{{"myapp"}},
		},
		{
			name: "HIP-0025 example: nginx+rabbitmq -> bar -> foo",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name: "foo",
					Annotations: map[string]string{
						AnnotationDependsOnSubcharts: `["bar", "rabbitmq"]`,
					},
					Dependencies: []*chart.Dependency{
						{Name: "nginx"},
						{Name: "rabbitmq"},
						{
							Name:      "bar",
							DependsOn: []string{"nginx", "rabbitmq"},
						},
					},
				},
			},
			expectedBatch: [][]string{
				{"nginx", "rabbitmq"},
				{"bar"},
				{"foo"},
			},
		},
		{
			name: "depends-on with alias",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name: "parent",
					Dependencies: []*chart.Dependency{
						{Name: "postgresql", Alias: "db"},
						{
							Name:      "app",
							DependsOn: []string{"db"},
						},
					},
				},
			},
			expectedBatch: [][]string{
				{"db", "parent"},
				{"app"},
			},
		},
		{
			name: "circular dependency via DependsOn",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name: "parent",
					Dependencies: []*chart.Dependency{
						{Name: "A", DependsOn: []string{"B"}},
						{Name: "B", DependsOn: []string{"A"}},
					},
				},
			},
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name: "unknown dependency reference",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name: "parent",
					Dependencies: []*chart.Dependency{
						{Name: "app", DependsOn: []string{"nonexistent"}},
					},
				},
			},
			expectError:   true,
			errorContains: "not a known dependency",
		},
		{
			name: "unknown annotation reference",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name: "parent",
					Annotations: map[string]string{
						AnnotationDependsOnSubcharts: `["ghost"]`,
					},
					Dependencies: []*chart.Dependency{
						{Name: "app"},
					},
				},
			},
			expectError:   true,
			errorContains: "not a known dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag, err := BuildSubchartDAG(tt.chart)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				batches, err := dag.Batches()
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBatch, batches)
			}
		})
	}
}
