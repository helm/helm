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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"

	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

// The custom-readiness annotation keys' canonical home is
// pkg/release/v1/util, next to the other sequencing annotations, so the pure
// plan builder (pkg/release/v1/sequence) can read them without importing
// pkg/kube. These value-identical aliases keep existing kube callers working.
const (
	// AnnotationReadinessSuccess declares custom readiness success conditions.
	AnnotationReadinessSuccess = releaseutil.AnnotationReadinessSuccess

	// AnnotationReadinessFailure declares custom readiness failure conditions.
	// Failure conditions take precedence over success conditions.
	AnnotationReadinessFailure = releaseutil.AnnotationReadinessFailure
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

// ExpressionWarning describes a readiness expression that could not be
// evaluated against the observed object state and was therefore treated as
// "condition not met" rather than failing the whole wait.
type ExpressionWarning struct {
	// Expression is the original annotation expression, e.g. `{.phase} > "Ready"`.
	Expression string
	// Detail is a human-readable reason, including the observed value.
	Detail string
}

// errIncomparableOrdering marks a comparison that cannot succeed against the
// observed values: an ordering operator (<, <=, >, >=) applied to operands
// that are not both numeric. The actual value's type is only known at runtime,
// so lint cannot always catch this. EvaluateCustomReadiness downgrades it to
// an ExpressionWarning — the expression counts as "condition not met",
// mirroring how a missing status key is treated — instead of returning an
// error that would abort the entire wait.
var errIncomparableOrdering = errors.New("ordering operators (<, <=, >, >=) require numeric values")

// EvaluateCustomReadiness evaluates custom readiness expressions against a
// resource's .status field.
//
// When both successExprs and failureExprs are empty or nil, or when only one
// side is provided, the caller should fall back to kstatus and useKstatus will
// be returned as true.
//
// Expressions that cannot be evaluated against the observed values (an
// ordering operator applied to non-numeric operands) are treated as
// "condition not met" and reported in the returned warnings rather than as an
// error. Expressions after the first met condition are not evaluated. All
// other evaluation problems (invalid JSONPath, unsupported operator) are
// returned as errors.
func EvaluateCustomReadiness(obj *unstructured.Unstructured, successExprs, failureExprs []string) (ReadinessStatus, bool, []ExpressionWarning, error) {
	hasSuccess := len(successExprs) > 0
	hasFailure := len(failureExprs) > 0

	if !hasSuccess || !hasFailure {
		// Partial annotations: fall back to kstatus. The warning for this
		// case is emitted once per batch by warnIfPartialReadinessAnnotations
		// in the sequencing layer, not here (which runs on every poll tick).
		return ReadinessPending, true, nil, nil
	}

	statusObj, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return ReadinessPending, false, nil, nil
	}

	statusWrapper := map[string]any{"status": statusObj}

	var warnings []ExpressionWarning

	for _, expr := range failureExprs {
		met, err := evaluateExpression(statusWrapper, expr)
		if err != nil {
			if errors.Is(err, errIncomparableOrdering) {
				warnings = append(warnings, ExpressionWarning{Expression: expr, Detail: err.Error()})
				continue
			}
			return ReadinessPending, false, warnings, fmt.Errorf("evaluating failure expression %q: %w", expr, err)
		}
		if met {
			return ReadinessFailed, false, warnings, nil
		}
	}

	for _, expr := range successExprs {
		met, err := evaluateExpression(statusWrapper, expr)
		if err != nil {
			if errors.Is(err, errIncomparableOrdering) {
				warnings = append(warnings, ExpressionWarning{Expression: expr, Detail: err.Error()})
				continue
			}
			return ReadinessPending, false, warnings, fmt.Errorf("evaluating success expression %q: %w", expr, err)
		}
		if met {
			return ReadinessReady, false, warnings, nil
		}
	}

	return ReadinessPending, false, warnings, nil
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

// ValidateReadinessExpressions parses a readiness annotation value (a JSON
// array of "{<jsonpath>} <op> <value>" expressions) and verifies that every
// expression is well-formed and compiles to a valid JSONPath. It does not
// evaluate anything against a live object — it is the static-validation entry
// point used by `helm lint` so malformed expressions are caught at author time
// rather than silently failing readiness at install time. An empty or absent
// annotation is valid (returns nil). It also rejects ordering operators
// (<, <=, >, >=) whose comparison value is not numeric, since such a
// comparison can never evaluate at runtime.
func ValidateReadinessExpressions(annotation string) error {
	exprs, err := parseReadinessExpressions(annotation)
	if err != nil {
		return err
	}

	for _, expr := range exprs {
		path, op, val, err := parseReadinessExpression(expr)
		if err != nil {
			return err
		}
		template, err := readinessJSONPath(path)
		if err != nil {
			return err
		}
		if err := jsonpath.New("readiness").Parse(template); err != nil {
			return fmt.Errorf("invalid JSONPath %q: %w", template, err)
		}
		// An ordering comparison can only ever succeed numerically, and the
		// literal's type is statically known: a non-numeric comparison value
		// with <, <=, >, >= is a definite authoring error.
		switch op {
		case "<", "<=", ">", ">=":
			if _, ok := tryParseFloat(trimReadinessValue(val)); !ok {
				return fmt.Errorf("expression %q: ordering operator %q requires a numeric comparison value, got %s", expr, op, val)
			}
		}
	}

	return nil
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
	case strings.HasPrefix(path, ".status.") || strings.HasPrefix(path, ".status["):
		// Already rooted at .status — avoid double-prefixing
		return "{" + path + "}", nil
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
		return "", "", "", errors.New("expression cannot be empty")
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
				return false, fmt.Errorf("cannot compare %q %s %q: %w", actual, op, expected, errIncomparableOrdering)
			}
		}
	}

	switch op {
	case "==":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	default:
		return false, fmt.Errorf("cannot compare %q %s %q: %w", actual, op, expected, errIncomparableOrdering)
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
