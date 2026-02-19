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
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
)

const (
	// AnnotationReadinessSuccess declares custom readiness success conditions.
	// Value is a newline-separated list of JSONPath expressions of the form:
	//   {.fieldPath} <operator> <value>
	// If ANY condition is true, the resource is considered ready.
	AnnotationReadinessSuccess = "helm.sh/readiness-success"

	// AnnotationReadinessFailure declares custom readiness failure conditions.
	// Value is a newline-separated list of JSONPath expressions.
	// If ANY condition is true, the resource is considered failed.
	// Failure conditions take precedence over success conditions.
	AnnotationReadinessFailure = "helm.sh/readiness-failure"
)

// ReadinessStatus represents the evaluated readiness of a resource.
type ReadinessStatus int

const (
	// ReadinessPending means neither success nor failure conditions are met.
	ReadinessPending ReadinessStatus = iota
	// ReadinessReady means at least one success condition is true.
	ReadinessReady
	// ReadinessFailed means at least one failure condition is true.
	ReadinessFailed
)

func (r ReadinessStatus) String() string {
	switch r {
	case ReadinessPending:
		return "Pending"
	case ReadinessReady:
		return "Ready"
	case ReadinessFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// EvaluateCustomReadiness evaluates custom readiness conditions from annotations.
//
// Both successExprs and failureExprs are slices of expression strings in the form:
//
//	{.fieldPath} <operator> <value>
//
// The JSONPath is scoped to .status — {.phase} evaluates against .status.phase.
//
// Returns:
//   - ReadinessStatus: the evaluated status (Pending/Ready/Failed)
//   - useKstatus bool: true when custom evaluation is not applicable and the caller
//     should fall back to kstatus. This happens when either successExprs or
//     failureExprs is nil/empty (only one annotation present).
//   - error: parsing errors (not "condition not met", which returns Pending)
//
// Evaluation order: failure conditions are checked first. If any failure condition
// is true → Failed. Then success conditions — if any is true → Ready. Otherwise → Pending.
func EvaluateCustomReadiness(obj *unstructured.Unstructured, successExprs, failureExprs []string) (ReadinessStatus, bool, error) {
	hasSuccess := len(successExprs) > 0
	hasFailure := len(failureExprs) > 0

	if !hasSuccess || !hasFailure {
		// Only one annotation present — fall back to kstatus with a warning.
		if hasSuccess || hasFailure {
			slog.Warn("only one custom readiness annotation present; falling back to kstatus",
				"resource", obj.GetName(),
				"hasReadinessSuccess", hasSuccess,
				"hasReadinessFailure", hasFailure,
			)
		}
		return ReadinessPending, true, nil
	}

	// Get .status as map for JSONPath evaluation.
	statusObj, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		// Missing status — treat as not-ready, not an error.
		return ReadinessPending, false, nil
	}

	// Wrap status in a map to allow {.fieldName} queries.
	statusWrapper := map[string]interface{}{"status": statusObj}

	// Check failure conditions first (precedence over success).
	for _, expr := range failureExprs {
		met, err := evaluateExpression(statusWrapper, expr)
		if err != nil {
			return ReadinessPending, false, fmt.Errorf("evaluating failure expression %q: %w", expr, err)
		}
		if met {
			return ReadinessFailed, false, nil
		}
	}

	// Check success conditions — OR semantics (any true → ready).
	for _, expr := range successExprs {
		met, err := evaluateExpression(statusWrapper, expr)
		if err != nil {
			return ReadinessPending, false, fmt.Errorf("evaluating success expression %q: %w", expr, err)
		}
		if met {
			return ReadinessReady, false, nil
		}
	}

	return ReadinessPending, false, nil
}

// evaluateExpression evaluates a single readiness expression against obj.
// Expression format: {.fieldPath} <operator> <value>
// Returns true if the condition is met, false if not met or field is missing.
func evaluateExpression(obj map[string]interface{}, expr string) (bool, error) {
	path, op, rawVal, err := parseReadinessExpression(expr)
	if err != nil {
		return false, err
	}

	// Build a JSONPath query against statusWrapper (path like ".phase" → "status.phase")
	jp := jsonpath.New("readiness")
	// JSONPath template: {.status.fieldName}
	template := "{.status" + path + "}"
	if err := jp.Parse(template); err != nil {
		return false, fmt.Errorf("invalid JSONPath %q: %w", template, err)
	}

	var buf bytes.Buffer
	if err := jp.Execute(&buf, obj); err != nil {
		// Field not found — treat as not-ready (not an error).
		return false, nil
	}

	actualVal := buf.String()
	return compareValues(actualVal, op, rawVal)
}

// operatorRegexp matches a comparison operator with surrounding optional whitespace
// at the beginning of a string: ==, !=, <=, >=, <, >
var operatorRegexp = regexp.MustCompile(`^(==|!=|<=|>=|<|>)\s+`)

// parseReadinessExpression parses "{.fieldPath} <operator> <value>" into components.
// Returns the JSONPath path (e.g., ".phase"), operator (e.g., "=="), and raw value string.
func parseReadinessExpression(expr string) (path, op, val string, err error) {
	// Find the JSONPath portion: {.something}
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(expr, "{") {
		return "", "", "", fmt.Errorf("expression must start with {.path}: %q", expr)
	}
	closeBrace := strings.Index(expr, "}")
	if closeBrace < 0 {
		return "", "", "", fmt.Errorf("expression missing closing }: %q", expr)
	}
	// path is contents of {}, e.g. ".phase"
	path = expr[1:closeBrace]
	rest := strings.TrimSpace(expr[closeBrace+1:])

	// Find operator at start of rest
	loc := operatorRegexp.FindStringSubmatchIndex(rest)
	if loc == nil {
		return "", "", "", fmt.Errorf("expression missing operator (==, !=, <, <=, >, >=): %q", expr)
	}
	op = rest[loc[2]:loc[3]] // capture group 1
	val = strings.TrimSpace(rest[loc[1]:])
	return path, op, val, nil
}

// compareValues compares actual (string from JSONPath) to expected (raw annotation value).
// Supports string, numeric, and boolean comparisons.
func compareValues(actual, op, expected string) (bool, error) {
	// Remove quotes from expected string values.
	expectedTrimmed := strings.Trim(expected, `"'`)
	actualTrimmed := actual

	// Try numeric comparison.
	actualFloat, actualIsNum := tryParseFloat(actualTrimmed)
	expectedFloat, expectedIsNum := tryParseFloat(expectedTrimmed)
	if actualIsNum && expectedIsNum {
		return compareNumeric(actualFloat, op, expectedFloat)
	}

	// Boolean comparison.
	if expected == "true" || expected == "false" {
		actualBool := actual == "true"
		expectedBool := expected == "true"
		switch op {
		case "==":
			return actualBool == expectedBool, nil
		case "!=":
			return actualBool != expectedBool, nil
		default:
			return false, fmt.Errorf("operator %q not supported for boolean values", op)
		}
	}

	// String comparison.
	switch op {
	case "==":
		return actualTrimmed == expectedTrimmed, nil
	case "!=":
		return actualTrimmed != expectedTrimmed, nil
	default:
		return false, fmt.Errorf("operator %q not supported for string values", op)
	}
}

func tryParseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func compareNumeric(a float64, op string, b float64) (bool, error) {
	switch op {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}
