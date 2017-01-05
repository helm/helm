/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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

package strvals

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
)

// ErrNotList indicates that a non-list was treated as a list.
var ErrNotList = errors.New("not a list")

// ToYAML takes a string of arguments and converts to a YAML document.
func ToYAML(s string) (string, error) {
	m, err := Parse(s)
	if err != nil {
		return "", err
	}
	d, err := yaml.Marshal(m)
	return string(d), err
}

// Parse parses a set line.
//
// A set line is of the form name1=value1,name2=value2
func Parse(s string) (map[string]interface{}, error) {
	vals := map[string]interface{}{}
	scanner := bytes.NewBufferString(s)
	t := newParser(scanner, vals)
	err := t.parse()
	return vals, err
}

//ParseInto parses a strvals line and merges the result into dest.
//
// If the strval string has a key that exists in dest, it overwrites the
// dest version.
func ParseInto(s string, dest map[string]interface{}) error {
	scanner := bytes.NewBufferString(s)
	t := newParser(scanner, dest)
	return t.parse()
}

// parser is a simple parser that takes a strvals line and parses it into a
// map representation.
type parser struct {
	sc   *bytes.Buffer
	data map[string]interface{}
}

func newParser(sc *bytes.Buffer, data map[string]interface{}) *parser {
	return &parser{sc: sc, data: data}
}

func (t *parser) parse() error {
	for {
		err := t.key(t.data)
		if err == nil {
			continue
		}
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func runeSet(r []rune) map[rune]bool {
	s := make(map[rune]bool, len(r))
	for _, rr := range r {
		s[rr] = true
	}
	return s
}

func (t *parser) key(data map[string]interface{}) error {
	stop := runeSet([]rune{'=', ',', '.'})
	for {
		switch k, last, err := runesUntil(t.sc, stop); {
		case err != nil:
			if len(k) == 0 {
				return err
			}
			return fmt.Errorf("key %q has no value", string(k))
			//set(data, string(k), "")
			//return err
		case last == '=':
			//End of key. Consume =, Get value.
			// FIXME: Get value list first
			vl, e := t.valList()
			switch e {
			case nil:
				set(data, string(k), vl)
				return nil
			case io.EOF:
				set(data, string(k), "")
				return e
			case ErrNotList:
				v, e := t.val()
				set(data, string(k), typedVal(v))
				return e
			default:
				return e
			}

		case last == ',':
			// No value given. Set the value to empty string. Return error.
			set(data, string(k), "")
			return fmt.Errorf("key %q has no value (cannot end with ,)", string(k))
		case last == '.':
			// First, create or find the target map.
			inner := map[string]interface{}{}
			if _, ok := data[string(k)]; ok {
				inner = data[string(k)].(map[string]interface{})
			}

			// Recurse
			e := t.key(inner)
			if len(inner) == 0 {
				return fmt.Errorf("key map %q has no value", string(k))
			}
			set(data, string(k), inner)
			return e
		}
	}
}

func set(data map[string]interface{}, key string, val interface{}) {
	// If key is empty, don't set it.
	if len(key) == 0 {
		return
	}
	data[key] = val
}

func (t *parser) val() ([]rune, error) {
	stop := runeSet([]rune{','})
	v, _, err := runesUntil(t.sc, stop)
	return v, err
}

func (t *parser) valList() ([]interface{}, error) {
	r, _, e := t.sc.ReadRune()
	if e != nil {
		return []interface{}{}, e
	}

	if r != '{' {
		t.sc.UnreadRune()
		return []interface{}{}, ErrNotList
	}

	list := []interface{}{}
	stop := runeSet([]rune{',', '}'})
	for {
		switch v, last, err := runesUntil(t.sc, stop); {
		case err != nil:
			if err == io.EOF {
				err = errors.New("list must terminate with '}'")
			}
			return list, err
		case last == '}':
			// If this is followed by ',', consume it.
			if r, _, e := t.sc.ReadRune(); e == nil && r != ',' {
				t.sc.UnreadRune()
			}
			list = append(list, typedVal(v))
			return list, nil
		case last == ',':
			list = append(list, typedVal(v))
		}
	}
}

func runesUntil(in io.RuneReader, stop map[rune]bool) ([]rune, rune, error) {
	v := []rune{}
	for {
		switch r, _, e := in.ReadRune(); {
		case e != nil:
			return v, r, e
		case inMap(r, stop):
			return v, r, nil
		case r == '\\':
			next, _, e := in.ReadRune()
			if e != nil {
				return v, next, e
			}
			v = append(v, next)
		default:
			v = append(v, r)
		}
	}
}

func inMap(k rune, m map[rune]bool) bool {
	_, ok := m[k]
	return ok
}

func typedVal(v []rune) interface{} {
	val := string(v)
	if strings.EqualFold(val, "true") {
		return true
	}

	if strings.EqualFold(val, "false") {
		return false
	}

	if iv, err := strconv.ParseInt(val, 10, 64); err == nil {
		return iv
	}

	return val
}
