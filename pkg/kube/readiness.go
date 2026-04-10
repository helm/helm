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

package kube // import "helm.sh/helm/v4/pkg/kube"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
)

const (
	// AnnotationReadinessSuccess declares custom readiness success conditions.
	// Its value is a JSON array of expressions in the form:
	//   "{.fieldPath} <operator> <value>"
	// If any success condition evaluates to true, the resource is considered ready.
	AnnotationReadinessSuccess = "helm.sh/readiness-success"

	// AnnotationReadinessFailure declares custom readiness failure conditions.
	// Its value is a JSON array of expressions in the form:
	//   "{.fieldPath} <operator> <value>"
	// If any failure condition evaluates to true, the resource is considered failed.
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

// EvaluateCustomReadiness evaluates custom readiness expressions against a
// resource's .status field.
//
// When both successExprs and failureExprs are empty or nil, or when only one
// side is provided, the caller should fall back to kstatus and useKstatus will
// be returned as true.
func EvaluateCustomReadiness(obj *unstructured.Unstructured, successExprs, failureExprs []string) (ReadinessStatus, bool, error) {
	hasSuccess := len(successExprs) > 0
	hasFailure := len(failureExprs) > 0

	if !hasSuccess || !hasFailure {
		if hasSuccess || hasFailure {
			slog.Warn(
				"only one custom readiness annotation present; falling back to kstatus",
				"resource", obj.GetName(),
				"hasReadinessSuccess", hasSuccess,
				"hasReadinessFailure", hasFailure,
			)
		}
		return ReadinessPending, true, nil
	}

	statusObj, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return ReadinessPending, false, nil
	}

	statusWrapper := map[string]any{"status": statusObj}

	for _, expr := range failureExprs {
		met, err := evaluateExpression(statusWrapper, expr)
		if err != nil {
			return ReadinessPending, false, fmt.Errorf("evaluating failure expression %q: %w", expr, err)
		}
		if met {
			return ReadinessFailed, false, nil
		}
	}

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

func parseReadinessExpressions(annotation string) ([]string, error) {
	annotation = strings.TrimSpace(annotation)
	if annotation == "" {
		return nil, nil
	}

	var exprs []string
	if err := json.Unmarshal([]byte(annotation), &exprs); err != nil {
		return nil, fmt.Errorf("parsing readiness annotation JSON: %w", err)
	}

	for i, expr := range exprs {
		exprs[i] = strings.TrimSpace(expr)
	}

	return exprs, nil
}

func evaluateExpression(obj map[string]any, expr string) (bool, error) {
	path, op, rawVal, err := parseReadinessExpression(expr)
	if err != nil {
		return false, err
	}

	template, err := readinessJSONPath(path)
	if err != nil {
		return false, err
	}

	jp := jsonpath.New("readiness")
	if err := jp.Parse(template); err != nil {
		return false, fmt.Errorf("invalid JSONPath %q: %w", template, err)
	}

	var buf bytes.Buffer
	if err := jp.Execute(&buf, obj); err != nil {
		return false, nil
	}

	return compareValues(strings.TrimSpace(buf.String()), op, rawVal)
}

func readinessJSONPath(path string) (string, error) {
	switch {
	case strings.HasPrefix(path, "."):
		return "{.status" + path + "}", nil
	case strings.HasPrefix(path, "["):
		return "{.status" + path + "}", nil
	default:
		return "", fmt.Errorf("invalid JSONPath %q: path must start with . or [", path)
	}
}

// parseReadinessExpression parses "{<jsonpath>} <operator> <value>" into its parts.
func parseReadinessExpression(expr string) (path, op, val string, err error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", "", "", fmt.Errorf("expression cannot be empty")
	}
	if !strings.HasPrefix(expr, "{") {
		return "", "", "", fmt.Errorf("expression must start with {<jsonpath>}: %q", expr)
	}

	closeBrace := strings.Index(expr, "}")
	if closeBrace < 0 {
		return "", "", "", fmt.Errorf("expression missing closing }: %q", expr)
	}

	path = strings.TrimSpace(expr[1:closeBrace])
	if path == "" {
		return "", "", "", fmt.Errorf("expression missing JSONPath: %q", expr)
	}

	rest := strings.TrimSpace(expr[closeBrace+1:])
	if rest == "" {
		return "", "", "", fmt.Errorf("expression missing operator and value: %q", expr)
	}

	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", "", "", fmt.Errorf("expression missing operator: %q", expr)
	}

	switch parts[0] {
	case "==", "!=", "<", "<=", ">", ">=":
		op = parts[0]
	default:
		return "", "", "", fmt.Errorf("unsupported operator %q in expression %q", parts[0], expr)
	}

	val = strings.TrimSpace(rest[len(op):])
	if val == "" {
		return "", "", "", fmt.Errorf("expression missing comparison value: %q", expr)
	}

	return path, op, val, nil
}

func compareValues(actual, op, expected string) (bool, error) {
	actual = strings.TrimSpace(actual)
	expected = trimReadinessValue(strings.TrimSpace(expected))

	if actualFloat, ok := tryParseFloat(actual); ok {
		if expectedFloat, ok := tryParseFloat(expected); ok {
			return compareNumeric(actualFloat, op, expectedFloat)
		}
	}

	if actualBool, actualIsBool := tryParseBool(actual); actualIsBool {
		if expectedBool, expectedIsBool := tryParseBool(expected); expectedIsBool {
			switch op {
			case "==":
				return actualBool == expectedBool, nil
			case "!=":
				return actualBool != expectedBool, nil
			default:
				return false, fmt.Errorf("operator %q not supported for boolean values", op)
			}
		}
	}

	switch op {
	case "==":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %q not supported for string values", op)
	}
}

func trimReadinessValue(value string) string {
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			return value[1 : len(value)-1]
		}
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func tryParseFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func tryParseBool(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(value)
	return parsed, err == nil
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
