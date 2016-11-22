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
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
)

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
	t.parse()
	return vals, nil
}

//ParseInto parses a strvals line and merges the result into dest.
//
// If the strval string has a key that exists in dest, it overwrites the
// dest version.
func ParseInto(s string, dest map[string]interface{}) error {
	scanner := bytes.NewBufferString(s)
	t := newParser(scanner, dest)
	t.parse()
	return nil
}

// parser is a simple parser that takes a strvals line and parses it into a
// map representation.
type parser struct {
	sc      *bytes.Buffer
	data    map[string]interface{}
	carryOn bool
}

func newParser(sc *bytes.Buffer, data map[string]interface{}) *parser {
	return &parser{sc: sc, data: data, carryOn: true}
}

func (t *parser) parse() error {
	// Starting state is consume key.
	for t.sc.Len() > 0 {
		t.key(t.data)
	}
	return nil
}

func (t *parser) key(data map[string]interface{}) {
	k := []rune{}
	for {
		switch r, _, e := t.sc.ReadRune(); {
		case e != nil:
			set(data, string(k), "")
			return
		case r == '\\':
			//Escape char. Consume next and append.
			next, _, e := t.sc.ReadRune()
			if e != nil {
				return
			}
			k = append(k, next)
		case r == '=':
			//End of key. Consume =, Get value.
			v := t.val()
			set(data, string(k), typedVal(v))
			return
		case r == ',':
			// No value given. Set the value to empty string.
			set(data, string(k), "")
			return
		case r == '.':
			// This is the difficult case. Now we're working with a subkey.

			// First, create or find the target map.
			inner := map[string]interface{}{}
			if _, ok := data[string(k)]; ok {
				inner = data[string(k)].(map[string]interface{})
			}

			// Recurse
			t.key(inner)
			set(data, string(k), inner)
			return
		default:
			k = append(k, r)
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

func (t *parser) val() []rune {
	v := []rune{}
	for {
		switch r, _, e := t.sc.ReadRune(); {
		case e != nil:
			// End of input or error with reader stops value parsing.
			return v
		case r == '\\':
			//Escape char. Consume next and append.
			next, _, e := t.sc.ReadRune()
			if e != nil {
				return v
			}
			v = append(v, next)
		case r == ',':
			//End of key. Consume ',' and return.
			return v
		default:
			v = append(v, r)
		}
	}
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
