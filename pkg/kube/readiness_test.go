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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeUnstructured(statusFields map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-resource")
	if statusFields != nil {
		if err := unstructured.SetNestedField(obj.Object, statusFields, "status"); err != nil {
			panic(err)
		}
	}
	return obj
}

func TestEvaluateReadiness_BothNil_UsesKstatus(t *testing.T) {
	obj := makeUnstructured(nil)
	result, useKstatus, err := EvaluateCustomReadiness(obj, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !useKstatus {
		t.Error("expected useKstatus=true when no custom conditions")
	}
	_ = result
}

func TestEvaluateReadiness_OnlySuccess_FallsBackToKstatus(t *testing.T) {
	obj := makeUnstructured(nil)
	successExprs := []string{`{.ready} == true`}
	result, useKstatus, err := EvaluateCustomReadiness(obj, successExprs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !useKstatus {
		t.Error("expected useKstatus=true (single annotation → fallback)")
	}
	_ = result
}

func TestEvaluateReadiness_OnlyFailure_FallsBackToKstatus(t *testing.T) {
	obj := makeUnstructured(nil)
	failureExprs := []string{`{.failed} == true`}
	result, useKstatus, err := EvaluateCustomReadiness(obj, nil, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !useKstatus {
		t.Error("expected useKstatus=true (single annotation → fallback)")
	}
	_ = result
}

func TestEvaluateReadiness_SuccessTrue(t *testing.T) {
	obj := makeUnstructured(map[string]interface{}{
		"succeeded": int64(1),
	})
	successExprs := []string{`{.succeeded} == 1`}
	failureExprs := []string{`{.failed} == true`}
	result, useKstatus, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if useKstatus {
		t.Error("expected useKstatus=false when both annotations provided")
	}
	if result != ReadinessReady {
		t.Errorf("expected Ready, got %v", result)
	}
}

func TestEvaluateReadiness_FailurePrecedesSuccess(t *testing.T) {
	// Both failure AND success conditions match. Failure takes precedence.
	obj := makeUnstructured(map[string]interface{}{
		"succeeded": int64(1),
		"failed":    true,
	})
	successExprs := []string{`{.succeeded} == 1`}
	failureExprs := []string{`{.failed} == true`}
	result, useKstatus, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if useKstatus {
		t.Error("expected useKstatus=false")
	}
	if result != ReadinessFailed {
		t.Errorf("expected Failed (failure takes precedence), got %v", result)
	}
}

func TestEvaluateReadiness_NeitherConditionMet(t *testing.T) {
	obj := makeUnstructured(map[string]interface{}{
		"succeeded": int64(0),
	})
	successExprs := []string{`{.succeeded} == 1`}
	failureExprs := []string{`{.failed} == true`}
	result, useKstatus, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if useKstatus {
		t.Error("expected useKstatus=false")
	}
	if result != ReadinessPending {
		t.Errorf("expected Pending (neither condition met), got %v", result)
	}
}

func TestEvaluateReadiness_ORSemantics_AnySuccessTrue(t *testing.T) {
	// OR semantics: if ANY success condition is true → ready
	obj := makeUnstructured(map[string]interface{}{
		"phase":    "Succeeded",
		"another":  "nope",
	})
	successExprs := []string{
		`{.another} == "yes"`,   // false
		`{.phase} == "Succeeded"`, // true
	}
	failureExprs := []string{`{.failed} == true`}
	result, _, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != ReadinessReady {
		t.Errorf("expected Ready (OR semantics), got %v", result)
	}
}

func TestEvaluateReadiness_MissingStatusField(t *testing.T) {
	// .nonexistent doesn't exist — should return Pending, not error
	obj := makeUnstructured(map[string]interface{}{})
	successExprs := []string{`{.nonexistent} == "yes"`}
	failureExprs := []string{`{.failed} == true`}
	result, _, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("expected no error for missing field, got: %v", err)
	}
	if result != ReadinessPending {
		t.Errorf("expected Pending for missing field, got %v", result)
	}
}

func TestEvaluateReadiness_StringComparison(t *testing.T) {
	obj := makeUnstructured(map[string]interface{}{
		"phase": "Running",
	})
	successExprs := []string{`{.phase} == "Running"`}
	failureExprs := []string{`{.phase} == "Failed"`}
	result, _, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != ReadinessReady {
		t.Errorf("expected Ready, got %v", result)
	}
}

func TestEvaluateReadiness_NotEqualOperator(t *testing.T) {
	obj := makeUnstructured(map[string]interface{}{
		"phase": "Running",
	})
	successExprs := []string{`{.phase} != "Failed"`}
	failureExprs := []string{`{.phase} == "Failed"`}
	result, _, err := EvaluateCustomReadiness(obj, successExprs, failureExprs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != ReadinessReady {
		t.Errorf("expected Ready, got %v", result)
	}
}

func TestParseReadinessExpression(t *testing.T) {
	tests := []struct {
		expr     string
		wantPath string
		wantOp   string
		wantVal  string
	}{
		{`{.phase} == "Running"`, ".phase", "==", `"Running"`},
		{`{.count} >= 3`, ".count", ">=", "3"},
		{`{.ready} != false`, ".ready", "!=", "false"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			path, op, val, err := parseReadinessExpression(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.wantPath {
				t.Errorf("path: want %q, got %q", tt.wantPath, path)
			}
			if op != tt.wantOp {
				t.Errorf("op: want %q, got %q", tt.wantOp, op)
			}
			if val != tt.wantVal {
				t.Errorf("val: want %q, got %q", tt.wantVal, val)
			}
		})
	}
}
