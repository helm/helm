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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseReadinessExpressions(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		expected    []ReadinessExpression
		expectError bool
	}{
		{
			name:     "empty string",
			raw:      "",
			expected: nil,
		},
		{
			name: "single expression",
			raw:  `["{.status.phase} == Running"]`,
			expected: []ReadinessExpression{
				{JSONPath: "{.status.phase}", Operator: "==", Value: "Running"},
			},
		},
		{
			name: "multiple expressions",
			raw:  `["{.status.replicas} >= 3", "{.status.phase} != Failed"]`,
			expected: []ReadinessExpression{
				{JSONPath: "{.status.replicas}", Operator: ">=", Value: "3"},
				{JSONPath: "{.status.phase}", Operator: "!=", Value: "Failed"},
			},
		},
		{
			name:        "invalid JSON",
			raw:         "not-json",
			expectError: true,
		},
		{
			name:        "invalid expression format",
			raw:         `["no-jsonpath here"]`,
			expectError: true,
		},
		{
			name:        "missing operator",
			raw:         `["{.status.phase} Running"]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseReadinessExpressions(tt.raw)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestEvaluateReadiness(t *testing.T) {
	tests := []struct {
		name           string
		obj            *unstructured.Unstructured
		expectedResult ReadinessResult
	}{
		{
			name: "no annotations - unknown",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
			expectedResult: ReadinessUnknown,
		},
		{
			name: "only success annotation - unknown (incomplete pair)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							AnnotationReadinessSuccess: `["{.status.phase} == Running"]`,
						},
					},
				},
			},
			expectedResult: ReadinessUnknown,
		},
		{
			name: "success conditions met",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							AnnotationReadinessSuccess: `["{.status.phase} == Running"]`,
							AnnotationReadinessFailure: `["{.status.phase} == Failed"]`,
						},
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			expectedResult: ReadinessReady,
		},
		{
			name: "failure condition met - takes precedence",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							AnnotationReadinessSuccess: `["{.status.phase} == Running"]`,
							AnnotationReadinessFailure: `["{.status.phase} == Failed"]`,
						},
					},
					"status": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
			expectedResult: ReadinessFailed,
		},
		{
			name: "pending - conditions not yet met",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							AnnotationReadinessSuccess: `["{.status.phase} == Running"]`,
							AnnotationReadinessFailure: `["{.status.phase} == Failed"]`,
						},
					},
					"status": map[string]interface{}{
						"phase": "Pending",
					},
				},
			},
			expectedResult: ReadinessPending,
		},
		{
			name: "numeric comparison",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							AnnotationReadinessSuccess: `["{.status.readyReplicas} >= 3"]`,
							AnnotationReadinessFailure: `["{.status.readyReplicas} < 0"]`,
						},
					},
					"status": map[string]interface{}{
						"readyReplicas": int64(3),
					},
				},
			},
			expectedResult: ReadinessReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := EvaluateReadiness(tt.obj)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		actual   string
		op       string
		expected string
		result   bool
	}{
		{"Running", "==", "Running", true},
		{"Running", "!=", "Failed", true},
		{"3", ">=", "3", true},
		{"5", ">", "3", true},
		{"2", "<", "3", true},
		{"3", "<=", "3", true},
		{"abc", "<", "bcd", true},
	}

	for _, tt := range tests {
		t.Run(tt.actual+" "+tt.op+" "+tt.expected, func(t *testing.T) {
			result, err := compareValues(tt.actual, tt.op, tt.expected)
			require.NoError(t, err)
			assert.Equal(t, tt.result, result)
		})
	}
}
