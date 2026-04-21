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
	"log"
	"strings"
	"unicode"

	"helm.sh/helm/v4/pkg/chart/common"
)

type conditionExpression interface {
	eval(conditionEvalContext) bool
}

type conditionEvalContext struct {
	values    common.Values
	condition string
	chartName string
	chartPath string
}

type conditionPath struct {
	path string
}

func (n conditionPath) eval(ctx conditionEvalContext) bool {
	v, err := ctx.values.PathValue(ctx.chartPath + n.path)
	if err != nil {
		if _, ok := err.(common.ErrNoValue); !ok {
			log.Printf("Warning: PathValue returned error %v", err)
		}
		return false
	}

	b, ok := v.(bool)
	if !ok {
		log.Printf("Warning: Condition path '%s' for chart %s returned non-bool value", n.path, ctx.chartName)
		return false
	}
	return b
}

type conditionNot struct {
	expr conditionExpression
}

func (n conditionNot) eval(ctx conditionEvalContext) bool {
	return !n.expr.eval(ctx)
}

type conditionAnd struct {
	left  conditionExpression
	right conditionExpression
}

func (n conditionAnd) eval(ctx conditionEvalContext) bool {
	if !n.left.eval(ctx) {
		return false
	}
	return n.right.eval(ctx)
}

type conditionOr struct {
	left  conditionExpression
	right conditionExpression
}

func (n conditionOr) eval(ctx conditionEvalContext) bool {
	if n.left.eval(ctx) {
		return true
	}
	return n.right.eval(ctx)
}

type conditionTokenType int

const (
	conditionTokenEOF conditionTokenType = iota
	conditionTokenLParen
	conditionTokenRParen
	conditionTokenAnd
	conditionTokenOr
	conditionTokenNot
	conditionTokenPath
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
		return conditionToken{typeID: conditionTokenLParen, pos: t.pos - 1}, nil
	case ')':
		t.pos++
		return conditionToken{typeID: conditionTokenRParen, pos: t.pos - 1}, nil
	case '!':
		t.pos++
		return conditionToken{typeID: conditionTokenNot, pos: t.pos - 1}, nil
	case '&':
		if t.peek('&') {
			t.pos += 2
			return conditionToken{typeID: conditionTokenAnd, pos: t.pos - 2}, nil
		}
		return conditionToken{}, fmt.Errorf("unexpected token '&' at position %d", t.pos)
	case '|':
		if t.peek('|') {
			t.pos += 2
			return conditionToken{typeID: conditionTokenOr, pos: t.pos - 2}, nil
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
		return nil, fmt.Errorf("unexpected token at position %d", p.current.pos)
	}
}

func IsConditionExpression(condition string) bool {
	trimmed := strings.TrimSpace(condition)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}

func EvaluateConditionExpression(condition string, cvals common.Values, cpath, chartName string) (bool, error) {
	expr, err := parseConditionExpression(condition)
	if err != nil {
		return false, err
	}

	ctx := conditionEvalContext{
		values:    cvals,
		condition: condition,
		chartName: chartName,
		chartPath: cpath,
	}
	return expr.eval(ctx), nil
}
