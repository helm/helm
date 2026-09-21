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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

const (
	// AnnotationReadinessSuccess declares custom readiness success conditions
	// as a JSON array of "{<jsonpath>} <operator> <value>" expressions.
	AnnotationReadinessSuccess = "helm.sh/readiness-success"

	// AnnotationReadinessFailure declares custom readiness failure conditions
	// as a JSON array of "{<jsonpath>} <operator> <value>" expressions. Failure
	// conditions take precedence over success conditions.
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
// an ExpressionWarning instead of aborting the entire wait.
var errIncomparableOrdering = errors.New("ordering operators (<, <=, >, >=) require numeric values")

// ParseReadinessExpressions parses a readiness annotation value as a JSON
// string array. Blank input, null, and an empty array are all represented as
// no expressions. Each parsed expression has surrounding whitespace removed.
func ParseReadinessExpressions(annotation string) ([]string, error) {
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
// evaluate anything against a live object. An empty or absent annotation is
// valid. Ordering operators (<, <=, >, >=) require a numeric comparison value.
func ValidateReadinessExpressions(annotation string) error {
	exprs, err := ParseReadinessExpressions(annotation)
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

		switch op {
		case "<", "<=", ">", ">=":
			if _, ok := tryParseFloat(trimReadinessValue(val)); !ok {
				return fmt.Errorf("expression %q: ordering operator %q requires a numeric comparison value, got %s", expr, op, val)
			}
		}
	}

	return nil
}

func readinessJSONPath(path string) (string, error) {
	switch {
	case strings.HasPrefix(path, ".status.") || strings.HasPrefix(path, ".status["):
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

func trimReadinessValue(value string) string {
	value = strings.TrimSpace(value)
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
