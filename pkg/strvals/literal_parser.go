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

package strvals

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/pkg/errors"
)

// ParseLiteral parses a set line interpreting the value as a literal string.
//
// A set line is of the form name1=value1
func ParseLiteral(s string) (map[string]interface{}, error) {
	vals := map[string]interface{}{}
	scanner := bytes.NewBufferString(s)
	t := newLiteralParser(scanner, vals)
	err := t.parse()
	return vals, err
}

// ParseLiteralInto parses a strvals line and merges the result into dest.
// The value is interpreted as a literal string.
//
// If the strval string has a key that exists in dest, it overwrites the
// dest version.
func ParseLiteralInto(s string, dest map[string]interface{}) error {
	scanner := bytes.NewBufferString(s)
	t := newLiteralParser(scanner, dest)
	return t.parse()
}

// literalParser is a simple parser that takes a strvals line and parses
// it into a map representation.
//
// Values are interpreted as a literal string.
//
// where sc is the source of the original data being parsed
// where data is the final parsed data from the parses with correct types
type literalParser struct {
	sc   *bytes.Buffer
	data map[string]interface{}
}

func newLiteralParser(sc *bytes.Buffer, data map[string]interface{}) *literalParser {
	return &literalParser{sc: sc, data: data}
}

func (t *literalParser) parse() error {
	for {
		err := t.key(t.data, 0)
		if err == nil {
			continue
		}
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func runesUntilLiteral(in io.RuneReader, stop map[rune]bool) ([]rune, rune, error) {
	v := []rune{}
	for {
		switch r, _, e := in.ReadRune(); {
		case e != nil:
			return v, r, e
		case inMap(r, stop):
			return v, r, nil
		default:
			v = append(v, r)
		}
	}
}

func (t *literalParser) key(data map[string]interface{}, nestedNameLevel int) (reterr error) {
	defer func() {
		if r := recover(); r != nil {
			reterr = fmt.Errorf("unable to parse key: %s", r)
		}
	}()
	stop := runeSet([]rune{'=', '[', '.'})
	for {
		switch key, lastRune, err := runesUntilLiteral(t.sc, stop); {
		case err != nil:
			if len(key) == 0 {
				return err
			}
			return errors.Errorf("key %q has no value", string(key))

		case lastRune == '=':
			// found end of key: swallow the '=' and get the value
			value, err := t.val()
			if err == nil && err != io.EOF {
				return err
			}
			set(data, string(key), string(value))
			return nil

		case lastRune == '.':
			// Check value name is within the maximum nested name level
			nestedNameLevel++
			if nestedNameLevel > MaxNestedNameLevel {
				return fmt.Errorf("value name nested level is greater than maximum supported nested level of %d", MaxNestedNameLevel)
			}

			// first, create or find the target map in the given data
			inner := map[string]interface{}{}
			if _, ok := data[string(key)]; ok {
				inner = data[string(key)].(map[string]interface{})
			}

			// recurse on sub-tree with remaining data
			err := t.key(inner, nestedNameLevel)
			if err == nil && len(inner) == 0 {
				return errors.Errorf("key map %q has no value", string(key))
			}
			if len(inner) != 0 {
				set(data, string(key), inner)
			}
			return err

		case lastRune == '[':
			// We are in a list index context, so we need to set an index.
			i, err := t.keyIndex()
			if err != nil {
				return errors.Wrap(err, "error parsing index")
			}
			kk := string(key)

			// find or create target list
			list := []interface{}{}
			if _, ok := data[kk]; ok {
				list = data[kk].([]interface{})
			}

			// now we need to get the value after the ]
			list, err = t.listItem(list, i, nestedNameLevel)
			set(data, kk, list)
			return err
		}
	}
}

func (t *literalParser) keyIndex() (int, error) {
	// First, get the key.
	stop := runeSet([]rune{']'})
	v, _, err := runesUntilLiteral(t.sc, stop)
	if err != nil {
		return 0, err
	}

	// v should be the index
	return strconv.Atoi(string(v))
}

func (t *literalParser) listItem(list []interface{}, i, nestedNameLevel int) ([]interface{}, error) {
	if i < 0 {
		return list, fmt.Errorf("negative %d index not allowed", i)
	}
	stop := runeSet([]rune{'[', '.', '='})

	switch key, lastRune, err := runesUntilLiteral(t.sc, stop); {
	case len(key) > 0:
		return list, errors.Errorf("unexpected data at end of array index: %q", key)

	case err != nil:
		return list, err

	case lastRune == '=':
		value, err := t.val()
		if err != nil && err != io.EOF {
			return list, err
		}
		return setIndex(list, i, string(value))

	case lastRune == '.':
		// we have a nested object. Send to t.key
		inner := map[string]interface{}{}
		if len(list) > i {
			var ok bool
			inner, ok = list[i].(map[string]interface{})
			if !ok {
				// We have indices out of order. Initialize empty value.
				list[i] = map[string]interface{}{}
				inner = list[i].(map[string]interface{})
			}
		}

		// recurse
		err := t.key(inner, nestedNameLevel)
		if err != nil {
			return list, err
		}
		return setIndex(list, i, inner)

	case lastRune == '[':
		// now we have a nested list. Read the index and handle.
		nextI, err := t.keyIndex()
		if err != nil {
			return list, errors.Wrap(err, "error parsing index")
		}
		var crtList []interface{}
		if len(list) > i {
			// If nested list already exists, take the value of list to next cycle.
			existed := list[i]
			if existed != nil {
				crtList = list[i].([]interface{})
			}
		}

		// Now we need to get the value after the ].
		list2, err := t.listItem(crtList, nextI, nestedNameLevel)
		if err != nil {
			return list, err
		}
		return setIndex(list, i, list2)

	default:
		return nil, errors.Errorf("parse error: unexpected token %v", lastRune)
	}
}

func (t *literalParser) val() ([]rune, error) {
	stop := runeSet([]rune{})
	v, _, err := runesUntilLiteral(t.sc, stop)
	return v, err
}
