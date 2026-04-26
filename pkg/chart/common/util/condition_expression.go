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
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"helm.sh/helm/v4/pkg/chart/common"
)

type conditionExpression interface {
	eval(conditionEvalContext) (bool, error)
}

type conditionEvalContext struct {
	values    common.Values
	chartName string
	chartPath string
}

type conditionPath struct {
	path string
}

func (n conditionPath) eval(ctx conditionEvalContext) (bool, error) {
	v, err := ctx.values.PathValue(ctx.chartPath + n.path)
	if err != nil {
		if _, ok := err.(common.ErrNoValue); !ok {
			slog.Warn("PathValue returned error", "error", err)
		}
		return false, err
	}

	b, ok := v.(bool)
	if !ok {
		slog.Warn("condition path returned non-bool value", "path", n.path, "chart", ctx.chartName)
		return false, fmt.Errorf("condition path returned non-bool value")
	}
	return b, nil
}

type conditionNot struct {
	expr conditionExpression
}

func (n conditionNot) eval(ctx conditionEvalContext) (bool, error) {
	result, err := n.expr.eval(ctx)
	if err != nil {
		return false, err
	}
	return !result, nil
}

type conditionAnd struct {
	left  conditionExpression
	right conditionExpression
}

func (n conditionAnd) eval(ctx conditionEvalContext) (bool, error) {
	left, err := n.left.eval(ctx)
	if err != nil {
		return false, err
	}
	if !left {
		return false, nil
	}
	return n.right.eval(ctx)
}

type conditionOr struct {
	left  conditionExpression
	right conditionExpression
}

func (n conditionOr) eval(ctx conditionEvalContext) (bool, error) {
	left, err := n.left.eval(ctx)
	if err != nil {
		return false, err
	}
	if left {
		return true, nil
	}
	return n.right.eval(ctx)
}

type conditionTokenType string

const (
	conditionTokenEOF    conditionTokenType = "EOF"
	conditionTokenLParen conditionTokenType = "Left Parenthesis"
	conditionTokenRParen conditionTokenType = "Right Parenthesis"
	conditionTokenAnd    conditionTokenType = "And"
	conditionTokenOr     conditionTokenType = "Or"
	conditionTokenNot    conditionTokenType = "Not"
	conditionTokenPath   conditionTokenType = "Path"
)

type conditionToken struct {
	typeID conditionTokenType
	value  string
	pos    int
}

type conditionTokenizer struct {
	input []rune
	pos   int
}

func newConditionTokenizer(input string) *conditionTokenizer {
	return &conditionTokenizer{input: []rune(input)}
}

func (t *conditionTokenizer) next() (conditionToken, error) {
	t.skipSpaces()
	if t.pos >= len(t.input) {
		return conditionToken{typeID: conditionTokenEOF, pos: t.pos}, nil
	}

	switch ch := t.input[t.pos]; ch {
	case '(':
		t.pos++
		return conditionToken{typeID: conditionTokenLParen, value: "(", pos: t.pos - 1}, nil
	case ')':
		t.pos++
		return conditionToken{typeID: conditionTokenRParen, value: ")", pos: t.pos - 1}, nil
	case '!':
		t.pos++
		return conditionToken{typeID: conditionTokenNot, value: "!", pos: t.pos - 1}, nil
	case '&':
		if t.peek('&') {
			t.pos += 2
			return conditionToken{typeID: conditionTokenAnd, value: "&&", pos: t.pos - 2}, nil
		}
		return conditionToken{}, fmt.Errorf("unexpected token '&' at position %d", t.pos)
	case '|':
		if t.peek('|') {
			t.pos += 2
			return conditionToken{typeID: conditionTokenOr, value: "||", pos: t.pos - 2}, nil
		}
		return conditionToken{}, fmt.Errorf("unexpected token '|' at position %d", t.pos)
	default:
		if isConditionPathChar(ch) {
			start := t.pos
			for t.pos < len(t.input) && isConditionPathChar(t.input[t.pos]) {
				t.pos++
			}
			return conditionToken{typeID: conditionTokenPath, value: string(t.input[start:t.pos]), pos: start}, nil
		}
		return conditionToken{}, fmt.Errorf("unexpected token '%c' at position %d", ch, t.pos)
	}
}

func (t *conditionTokenizer) skipSpaces() {
	for t.pos < len(t.input) && unicode.IsSpace(t.input[t.pos]) {
		t.pos++
	}
}

func (t *conditionTokenizer) peek(ch rune) bool {
	return t.pos+1 < len(t.input) && t.input[t.pos+1] == ch
}

func isConditionPathChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-' || ch == '.'
}

type conditionParser struct {
	tokenizer *conditionTokenizer
	current   conditionToken
}

func parseConditionExpression(input string) (conditionExpression, error) {
	p := &conditionParser{tokenizer: newConditionTokenizer(input)}
	if err := p.advance(); err != nil {
		return nil, err
	}

	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if p.current.typeID != conditionTokenEOF {
		return nil, fmt.Errorf("unexpected token '%s' at position %d", p.current.value, p.current.pos)
	}
	return expr, nil
}

func (p *conditionParser) advance() error {
	token, err := p.tokenizer.next()
	if err != nil {
		return err
	}
	p.current = token
	return nil
}

func (p *conditionParser) parseOr() (conditionExpression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.current.typeID == conditionTokenOr {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = conditionOr{left: left, right: right}
	}

	return left, nil
}

func (p *conditionParser) parseAnd() (conditionExpression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.current.typeID == conditionTokenAnd {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = conditionAnd{left: left, right: right}
	}

	return left, nil
}

func (p *conditionParser) parseUnary() (conditionExpression, error) {
	if p.current.typeID == conditionTokenNot {
		if err := p.advance(); err != nil {
			return nil, err
		}
		nested, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return conditionNot{expr: nested}, nil
	}

	return p.parsePrimary()
}

func (p *conditionParser) parsePrimary() (conditionExpression, error) {
	switch p.current.typeID {
	case conditionTokenPath:
		path := p.current.value
		if err := p.advance(); err != nil {
			return nil, err
		}
		return conditionPath{path: path}, nil
	case conditionTokenLParen:
		if err := p.advance(); err != nil {
			return nil, err
		}
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.current.typeID != conditionTokenRParen {
			return nil, fmt.Errorf("missing closing ')' at position %d", p.current.pos)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token %q (type %v) at position %d", p.current.value, p.current.typeID, p.current.pos)
	}
}

// IsConditionExpression reports whether condition uses the boolean-expression
// syntax understood by EvaluateConditionExpression.
//
// A condition expression must be wrapped in a single outer pair of
// parentheses after trimming whitespace, for example `(subchart.enabled &&
// !global.disabled)`. Inside the outer parentheses, operands are value paths
// made of letters, digits, `_`, `-`, and `.`, and they may be combined with
// `!`, `&&`, `||`, and nested parentheses.
func IsConditionExpression(condition string) bool {
	trimmed := strings.TrimSpace(condition)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}

// EvaluateConditionExpression parses and evaluates a condition expression
// against the provided values.
//
// The condition must follow the syntax recognized by IsConditionExpression:
// after trimming whitespace, the full expression is expected to be enclosed in
// outer parentheses, operands are value paths containing only letters,
// digits, `_`, `-`, and `.`, and operators are limited to `!`, `&&`, `||`,
// and nested parentheses. Evaluation returns an error if any referenced path
// is missing or resolves to a non-bool value.
func EvaluateConditionExpression(condition string, cvals common.Values, cpath, chartName string) (bool, error) {
	expr, err := parseConditionExpression(condition)
	if err != nil {
		return false, err
	}

	ctx := conditionEvalContext{
		values:    cvals,
		chartName: chartName,
		chartPath: cpath,
	}
	return expr.eval(ctx)
}
