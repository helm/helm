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
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
)

const (
	// AnnotationReadinessSuccess is the annotation key for custom readiness success conditions.
	// Value is a JSON array of "{.jsonpath} op value" expressions.
	AnnotationReadinessSuccess = "helm.sh/readiness-success"

	// AnnotationReadinessFailure is the annotation key for custom readiness failure conditions.
	// Value is a JSON array of "{.jsonpath} op value" expressions.
	// Failure takes precedence over success.
	AnnotationReadinessFailure = "helm.sh/readiness-failure"
)

// ReadinessResult represents the outcome of a custom readiness evaluation.
type ReadinessResult int

const (
	// ReadinessUnknown means readiness could not be determined (fall back to kstatus).
	ReadinessUnknown ReadinessResult = iota
	// ReadinessReady means all success conditions are met and no failure conditions triggered.
	ReadinessReady
	// ReadinessFailed means a failure condition was triggered.
	ReadinessFailed
	// ReadinessPending means no failure triggered but success conditions not yet met.
	ReadinessPending
)

// ReadinessExpression represents a parsed "{.jsonpath} op value" expression.
type ReadinessExpression struct {
	// JSONPath is the path expression scoped to the resource object (e.g., "{.status.phase}").
	JSONPath string
	// Operator is the comparison operator (==, !=, <, <=, >, >=).
	Operator string
	// Value is the expected scalar value to compare against.
	Value string
}

// ParseReadinessExpressions parses a JSON array of readiness expressions.
// Each expression has the format: "{.jsonpath} op value"
func ParseReadinessExpressions(raw string) ([]ReadinessExpression, error) {
	if raw == "" {
		return nil, nil
	}

	var exprs []string
	if err := json.Unmarshal([]byte(raw), &exprs); err != nil {
		return nil, fmt.Errorf("invalid readiness expression JSON: %w", err)
	}

	result := make([]ReadinessExpression, 0, len(exprs))
	for _, expr := range exprs {
		parsed, err := parseOneExpression(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid readiness expression %q: %w", expr, err)
		}
		result = append(result, parsed)
	}
	return result, nil
}

// parseOneExpression parses a single "{.jsonpath} op value" string.
func parseOneExpression(expr string) (ReadinessExpression, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ReadinessExpression{}, fmt.Errorf("empty expression")
	}

	// Find the end of the JSONPath (closing brace)
	braceEnd := strings.Index(expr, "}")
	if braceEnd < 0 || !strings.HasPrefix(expr, "{") {
		return ReadinessExpression{}, fmt.Errorf("expression must start with a JSONPath like {.status.phase}")
	}

	jp := expr[:braceEnd+1]
	rest := strings.TrimSpace(expr[braceEnd+1:])

	// Parse operator
	var op string
	for _, candidate := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		if strings.HasPrefix(rest, candidate) {
			op = candidate
			rest = strings.TrimSpace(rest[len(candidate):])
			break
		}
	}
	if op == "" {
		return ReadinessExpression{}, fmt.Errorf("no valid operator found (expected ==, !=, <, <=, >, >=)")
	}

	// Remaining is the value (trimmed)
	val := strings.TrimSpace(rest)
	if val == "" {
		return ReadinessExpression{}, fmt.Errorf("missing comparison value")
	}

	return ReadinessExpression{
		JSONPath: jp,
		Operator: op,
		Value:    val,
	}, nil
}

// EvaluateReadiness evaluates custom readiness for an unstructured Kubernetes object.
// Returns ReadinessUnknown if the object has no readiness annotations (or only one of the pair).
// Failure conditions take precedence over success conditions per the spec.
func EvaluateReadiness(obj *unstructured.Unstructured) (ReadinessResult, string, error) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ReadinessUnknown, "", nil
	}

	successRaw := annotations[AnnotationReadinessSuccess]
	failureRaw := annotations[AnnotationReadinessFailure]

	// Both must be present; if only one, warn and fall back
	hasSuccess := successRaw != ""
	hasFailure := failureRaw != ""

	if !hasSuccess && !hasFailure {
		return ReadinessUnknown, "", nil
	}
	if hasSuccess != hasFailure {
		which := AnnotationReadinessSuccess
		if hasFailure {
			which = AnnotationReadinessFailure
		}
		slog.Warn("only one readiness annotation present, both required; falling back to kstatus",
			"annotation", which,
			"resource", obj.GetName(),
		)
		return ReadinessUnknown, "incomplete readiness annotations", nil
	}

	// Parse failure expressions (evaluate first — failure takes precedence)
	failureExprs, err := ParseReadinessExpressions(failureRaw)
	if err != nil {
		return ReadinessUnknown, "", fmt.Errorf("resource %s: %w", obj.GetName(), err)
	}

	successExprs, err := ParseReadinessExpressions(successRaw)
	if err != nil {
		return ReadinessUnknown, "", fmt.Errorf("resource %s: %w", obj.GetName(), err)
	}

	// Check failure conditions first
	for _, expr := range failureExprs {
		match, err := evaluateExpression(obj.Object, expr)
		if err != nil {
			// If we can't evaluate, skip this expression
			continue
		}
		if match {
			return ReadinessFailed, fmt.Sprintf("failure condition met: %s %s %s", expr.JSONPath, expr.Operator, expr.Value), nil
		}
	}

	// Check success conditions
	allSuccess := true
	for _, expr := range successExprs {
		match, err := evaluateExpression(obj.Object, expr)
		if err != nil {
			allSuccess = false
			continue
		}
		if !match {
			allSuccess = false
		}
	}

	if allSuccess && len(successExprs) > 0 {
		return ReadinessReady, "all success conditions met", nil
	}

	return ReadinessPending, "waiting for success conditions", nil
}

// evaluateExpression evaluates a single readiness expression against a resource object.
func evaluateExpression(obj map[string]interface{}, expr ReadinessExpression) (bool, error) {
	// Parse and execute the JSONPath
	jp := jsonpath.New("readiness")
	if err := jp.Parse(expr.JSONPath); err != nil {
		return false, fmt.Errorf("invalid JSONPath %q: %w", expr.JSONPath, err)
	}

	results, err := jp.FindResults(obj)
	if err != nil {
		return false, fmt.Errorf("JSONPath %q evaluation failed: %w", expr.JSONPath, err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return false, fmt.Errorf("JSONPath %q returned no results", expr.JSONPath)
	}

	// Get the actual value as a string for comparison
	actual := fmt.Sprintf("%v", results[0][0].Interface())

	return compareValues(actual, expr.Operator, expr.Value)
}

// compareValues compares two string values using the given operator.
// Attempts numeric comparison first; falls back to string comparison.
func compareValues(actual, op, expected string) (bool, error) {
	// Try numeric comparison
	actualNum, aErr := strconv.ParseFloat(actual, 64)
	expectedNum, eErr := strconv.ParseFloat(expected, 64)
	if aErr == nil && eErr == nil {
		return compareNumeric(actualNum, op, expectedNum)
	}

	// String comparison (only == and != are valid for strings)
	switch op {
	case "==":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	case "<", "<=", ">", ">=":
		// For non-numeric values with ordering operators, compare lexicographically
		return compareLexicographic(actual, op, expected)
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

func compareNumeric(actual float64, op string, expected float64) (bool, error) {
	switch op {
	case "==":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	case "<":
		return actual < expected, nil
	case "<=":
		return actual <= expected, nil
	case ">":
		return actual > expected, nil
	case ">=":
		return actual >= expected, nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

func compareLexicographic(actual, op, expected string) (bool, error) {
	switch op {
	case "<":
		return actual < expected, nil
	case "<=":
		return actual <= expected, nil
	case ">":
		return actual > expected, nil
	case ">=":
		return actual >= expected, nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}
