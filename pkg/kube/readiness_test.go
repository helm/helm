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

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeUnstructuredWithStatus(t *testing.T, statusFields map[string]any) *unstructured.Unstructured {
	t.Helper()

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-resource")

	if statusFields != nil {
		require.NoError(t, unstructured.SetNestedField(obj.Object, statusFields, "status"))
	}

	return obj
}

func TestEvaluateReadiness_BothNil_UsesKstatus(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, nil)

	result, useKstatus, err := EvaluateCustomReadiness(obj, nil, nil)

	require.NoError(t, err)
	assert.True(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)

	result, useKstatus, err = EvaluateCustomReadiness(obj, []string{}, []string{})

	require.NoError(t, err)
	assert.True(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)
}

func TestEvaluateReadiness_OnlySuccess_FallsBackToKstatus(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, nil)

	result, useKstatus, err := EvaluateCustomReadiness(obj, []string{`{.ready} == true`}, nil)

	require.NoError(t, err)
	assert.True(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)
}

func TestEvaluateReadiness_OnlyFailure_FallsBackToKstatus(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, nil)

	result, useKstatus, err := EvaluateCustomReadiness(obj, nil, []string{`{.failed} == true`})

	require.NoError(t, err)
	assert.True(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)
}

func TestEvaluateReadiness_SuccessTrue(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"succeeded": int64(1),
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.succeeded} == 1`},
		[]string{`{.failed} >= 1`},
	)

	require.NoError(t, err)
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessReady, result)
}

func TestEvaluateReadiness_FailurePrecedesSuccess(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"succeeded": int64(1),
		"failed":    int64(1),
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.succeeded} == 1`},
		[]string{`{.failed} >= 1`},
	)

	require.NoError(t, err)
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessFailed, result)
}

func TestEvaluateReadiness_ORSemantics_AnySuccessTrue(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"phase":   "Succeeded",
		"another": "nope",
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.another} == "yes"`, `{.phase} == "Succeeded"`},
		[]string{`{.failed} >= 1`},
	)

	require.NoError(t, err)
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessReady, result)
}

func TestEvaluateReadiness_NeitherConditionMet(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"succeeded": int64(0),
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.succeeded} == 1`},
		[]string{`{.failed} >= 1`},
	)

	require.NoError(t, err)
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)
}

func TestEvaluateReadiness_MissingStatusField(t *testing.T) {
	t.Run("missing status object", func(t *testing.T) {
		obj := makeUnstructuredWithStatus(t, nil)

		result, useKstatus, err := EvaluateCustomReadiness(
			obj,
			[]string{`{.phase} == "Running"`},
			[]string{`{.failed} >= 1`},
		)

		require.NoError(t, err)
		assert.False(t, useKstatus)
		assert.Equal(t, ReadinessPending, result)
	})

	t.Run("missing status field", func(t *testing.T) {
		obj := makeUnstructuredWithStatus(t, map[string]any{})

		result, useKstatus, err := EvaluateCustomReadiness(
			obj,
			[]string{`{.phase} == "Running"`},
			[]string{`{.failed} >= 1`},
		)

		require.NoError(t, err)
		assert.False(t, useKstatus)
		assert.Equal(t, ReadinessPending, result)
	})
}

func TestEvaluateReadiness_NumericComparisonOperators(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		status map[string]any
	}{
		{name: "equal", expr: `{.succeeded} == 1`, status: map[string]any{"succeeded": int64(1)}},
		{name: "not equal", expr: `{.succeeded} != 2`, status: map[string]any{"succeeded": int64(1)}},
		{name: "less than", expr: `{.succeeded} < 2`, status: map[string]any{"succeeded": int64(1)}},
		{name: "less than or equal", expr: `{.succeeded} <= 1`, status: map[string]any{"succeeded": int64(1)}},
		{name: "greater than", expr: `{.succeeded} > 0`, status: map[string]any{"succeeded": int64(1)}},
		{name: "greater than or equal", expr: `{.succeeded} >= 1`, status: map[string]any{"succeeded": int64(1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := makeUnstructuredWithStatus(t, tt.status)

			result, useKstatus, err := EvaluateCustomReadiness(
				obj,
				[]string{tt.expr},
				[]string{`{.failed} >= 1`},
			)

			require.NoError(t, err)
			assert.False(t, useKstatus)
			assert.Equal(t, ReadinessReady, result)
		})
	}
}

func TestEvaluateReadiness_BooleanComparison(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		status map[string]any
	}{
		{name: "boolean equals", expr: `{.ready} == true`, status: map[string]any{"ready": true}},
		{name: "boolean not equals", expr: `{.ready} != false`, status: map[string]any{"ready": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := makeUnstructuredWithStatus(t, tt.status)

			result, useKstatus, err := EvaluateCustomReadiness(
				obj,
				[]string{tt.expr},
				[]string{`{.failed} == true`},
			)

			require.NoError(t, err)
			assert.False(t, useKstatus)
			assert.Equal(t, ReadinessReady, result)
		})
	}
}

func TestEvaluateReadiness_StringComparison(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"phase": "Running",
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.phase} == "Running"`},
		[]string{`{.phase} == "Failed"`},
	)

	require.NoError(t, err)
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessReady, result)

	result, useKstatus, err = EvaluateCustomReadiness(
		obj,
		[]string{`{.phase} != "Failed"`},
		[]string{`{.phase} == "Failed"`},
	)

	require.NoError(t, err)
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessReady, result)
}

func TestEvaluateReadiness_InvalidJSONPath(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"phase": "Running",
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.phase[} == "Running"`},
		[]string{`{.failed} >= 1`},
	)

	assert.ErrorContains(t, err, "invalid JSONPath")
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)
}

func TestEvaluateReadiness_InvalidOperator(t *testing.T) {
	obj := makeUnstructuredWithStatus(t, map[string]any{
		"phase": "Running",
	})

	result, useKstatus, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.phase} <> "Running"`},
		[]string{`{.failed} >= 1`},
	)

	assert.ErrorContains(t, err, "unsupported operator")
	assert.False(t, useKstatus)
	assert.Equal(t, ReadinessPending, result)
}

func TestParseReadinessExpression(t *testing.T) {
	t.Run("valid expressions", func(t *testing.T) {
		tests := []struct {
			name     string
			expr     string
			wantPath string
			wantOp   string
			wantVal  string
		}{
			{name: "string equality", expr: `{.phase} == "Running"`, wantPath: ".phase", wantOp: "==", wantVal: `"Running"`},
			{name: "numeric greater than or equal", expr: `{.count} >= 3`, wantPath: ".count", wantOp: ">=", wantVal: "3"},
			{name: "boolean not equal", expr: `{.ready} != false`, wantPath: ".ready", wantOp: "!=", wantVal: "false"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				path, op, val, err := parseReadinessExpression(tt.expr)

				require.NoError(t, err)
				assert.Equal(t, tt.wantPath, path)
				assert.Equal(t, tt.wantOp, op)
				assert.Equal(t, tt.wantVal, val)
			})
		}
	})

	t.Run("invalid expressions", func(t *testing.T) {
		tests := []struct {
			name        string
			expr        string
			wantErrPart string
		}{
			{name: "missing opening brace", expr: `.phase == "Running"`, wantErrPart: "must start"},
			{name: "missing closing brace", expr: `{.phase == "Running"`, wantErrPart: "missing closing"},
			{name: "missing operator", expr: `{.phase} "Running"`, wantErrPart: "unsupported operator"},
			{name: "unsupported operator", expr: `{.phase} <> "Running"`, wantErrPart: "unsupported operator"},
			{name: "missing value", expr: `{.phase} ==`, wantErrPart: "missing comparison value"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, _, err := parseReadinessExpression(tt.expr)
				assert.ErrorContains(t, err, tt.wantErrPart)
			})
		}
	})
}

func TestReadinessExpressionsJSON(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		exprs, err := parseReadinessExpressions(`[]`)

		require.NoError(t, err)
		assert.Empty(t, exprs)
	})

	t.Run("malformed annotation JSON", func(t *testing.T) {
		_, err := parseReadinessExpressions(`["{.ready} == true"`)
		assert.ErrorContains(t, err, "parsing readiness annotation JSON")
	})
}
