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
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/common"
)

func TestIsConditionExpression(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		want      bool
	}{
		{name: "trimmed outer parentheses", condition: " (enabled && !disabled) ", want: true},
		{name: "missing leading parenthesis", condition: "enabled && !disabled)", want: false},
		{name: "missing trailing parenthesis", condition: "(enabled && !disabled", want: false},
		{name: "plain path", condition: "enabled", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsConditionExpression(test.condition); got != test.want {
				t.Fatalf("IsConditionExpression(%q) = %t, want %t", test.condition, got, test.want)
			}
		})
	}
}

func TestParseConditionExpression(t *testing.T) {
	values := common.Values{
		"enabled":  true,
		"disabled": false,
		"other":    false,
		"nested": map[string]any{
			"enabled":  true,
			"disabled": false,
		},
	}

	tests := []struct {
		name       string
		expression string
		want       bool
	}{
		{
			name:       "and binds tighter than or",
			expression: "enabled || other && disabled",
			want:       true,
		},
		{
			name:       "parentheses override precedence",
			expression: "(enabled || other) && disabled",
			want:       false,
		},
		{
			name:       "whitespace and unary not",
			expression: " !nested.disabled && nested.enabled ",
			want:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expr, err := parseConditionExpression(test.expression)
			if err != nil {
				t.Fatalf("parseConditionExpression(%q) returned error: %v", test.expression, err)
			}

			got, err := expr.eval(conditionEvalContext{values: values, chartName: "chart"})
			if err != nil {
				t.Fatalf("expr.eval() returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("parseConditionExpression(%q) evaluated to %t, want %t", test.expression, got, test.want)
			}
		})
	}
}

func TestEvaluateConditionExpression(t *testing.T) {
	values := common.Values{
		"parent": map[string]any{
			"enabled": true,
			"guard":   false,
			"name":    "demo",
		},
		"fallback": true,
	}

	tests := []struct {
		name      string
		condition string
		chartPath string
		want      bool
		wantErr   string
	}{
		{
			name:      "uses chart path prefix",
			condition: "(enabled && !guard)",
			chartPath: "parent.",
			want:      true,
		},
		{
			name:      "short circuit or avoids missing path error",
			condition: "(enabled || missing)",
			chartPath: "parent.",
			want:      true,
		},
		{
			name:      "short circuit and avoids missing path error",
			condition: "(guard && missing)",
			chartPath: "parent.",
			want:      false,
		},
		{
			name:      "missing values return error",
			condition: "(missing || enabled)",
			chartPath: "parent.",
			wantErr:   "\"missing\" is not a value",
		},
		{
			name:      "non boolean values return error",
			condition: "(name || guard)",
			chartPath: "parent.",
			wantErr:   "condition path returned non-bool value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := EvaluateConditionExpression(test.condition, values, test.chartPath, "chart")
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("EvaluateConditionExpression(%q) returned nil error, want %q", test.condition, test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("EvaluateConditionExpression(%q) error = %q, want substring %q", test.condition, err.Error(), test.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("EvaluateConditionExpression(%q) returned error: %v", test.condition, err)
			}

			if got != test.want {
				t.Fatalf("EvaluateConditionExpression(%q) = %t, want %t", test.condition, got, test.want)
			}
		})
	}
}

func TestParseConditionExpressionErrors(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantErr    string
	}{
		{
			name:       "missing closing parenthesis",
			expression: "(enabled && (other || disabled)",
			wantErr:    "missing closing ')'",
		},
		{
			name:       "single ampersand",
			expression: "enabled & other",
			wantErr:    "unexpected token '&'",
		},
		{
			name:       "invalid character",
			expression: "enabled || $other",
			wantErr:    "unexpected token '$'",
		},
		{
			name:       "operator without right operand",
			expression: "enabled &&",
			wantErr:    "unexpected token \"\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseConditionExpression(test.expression)
			if err == nil {
				t.Fatalf("parseConditionExpression(%q) returned nil error", test.expression)
			}

			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("parseConditionExpression(%q) error = %q, want substring %q", test.expression, err.Error(), test.wantErr)
			}
		})
	}
}
