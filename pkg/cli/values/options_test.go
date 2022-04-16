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

package values

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/getter"
)

func TestMergeValues(t *testing.T) {
	nestedMap := map[string]interface{}{
		"foo": "bar",
		"baz": map[string]string{
			"cool": "stuff",
		},
	}
	anotherNestedMap := map[string]interface{}{
		"foo": "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	flatMap := map[string]interface{}{
		"foo": "bar",
		"baz": "stuff",
	}
	anotherFlatMap := map[string]interface{}{
		"testing": "fun",
	}

	testMap := mergeMaps(flatMap, nestedMap)
	equal := reflect.DeepEqual(testMap, nestedMap)
	if !equal {
		t.Errorf("Expected a nested map to overwrite a flat value. Expected: %v, got %v", nestedMap, testMap)
	}

	testMap = mergeMaps(nestedMap, flatMap)
	equal = reflect.DeepEqual(testMap, flatMap)
	if !equal {
		t.Errorf("Expected a flat value to overwrite a map. Expected: %v, got %v", flatMap, testMap)
	}

	testMap = mergeMaps(nestedMap, anotherNestedMap)
	equal = reflect.DeepEqual(testMap, anotherNestedMap)
	if !equal {
		t.Errorf("Expected a nested map to overwrite another nested map. Expected: %v, got %v", anotherNestedMap, testMap)
	}

	testMap = mergeMaps(anotherFlatMap, anotherNestedMap)
	expectedMap := map[string]interface{}{
		"testing": "fun",
		"foo":     "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected a map with different keys to merge properly with another map. Expected: %v, got %v", expectedMap, testMap)
	}
}

func TestReadFile(t *testing.T) {
	var p getter.Providers
	filePath := "%a.txt"
	_, err := readFile(filePath, p)
	if err == nil {
		t.Errorf("Expected error when has special strings")
	}
}

func TestMergeYaml(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		err    bool
		expect map[string]interface{}
	}{
		{"string value", "key: value1", false, map[string]interface{}{
			"key": "value1",
		}},
		{"int value", "key: 1000", false, map[string]interface{}{
			"key": 1000,
		}},
		{"nil value", "key: ~", false, map[string]interface{}{
			"key": nil,
		}},

		{"sub map", "key: {subkey: sub value}", false, map[string]interface{}{
			"key": map[string]interface{}{
				"subkey": "sub value",
			},
		}},

		{"empty document", "---", false, map[string]interface{}{}},
		{"empty document with newline", "---\n", false, map[string]interface{}{}},

		{"empty documents", "\n---\n", false, map[string]interface{}{}},

		{"multiple documents w/o first separator", "key1: value1\n---\nkey2: value2", false, map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}},
		{"multiple documents w/ first separator", "---\nkey1: value1\n---\nkey2: value2", false, map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}},
		{"override keys with string", "---\nkey1: value1\n---\nkey1: value2", false, map[string]interface{}{
			"key1": "value2",
		}},
		{"override keys with nil", "key1: value1\n---\nkey2: ", false, map[string]interface{}{
			"key1": "value1",
			"key2": nil,
		}},
		{"override keys with nil + additional separator", "key1: value1\n---\nkey2: \n---", false, map[string]interface{}{
			"key1": "value1",
			"key2": nil,
		}},
		{"override with map", "---\nfoo: {key: value}\n#---\nfoo: {key: value2}", false, map[string]interface{}{
			"foo": map[string]interface{}{
				"key": "value2",
			},
		}},

		{"yaml syntax error in 2nd document", "key1: value1\n---\nkey2 ", true, nil},

		{"7 yaml documents", "key1: value1\n---\nkey2: value2\n---\nkey3: value3\n---\nkey4: value4\n---\nkey5: value5\n---\nkey6: value6\n---\nkey7: value7\n---", false, map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
			"key6": "value6",
			"key7": "value7",
		}},

		{"comment in yaml separator", "---\nfoo: bar\n--- # with Comment\nbaz: biz", false, map[string]interface{}{
			"foo": "bar",
			"baz": "biz",
		}},

		{"yaml anchors", "---\nkey1: &value value\nkey2:\n- *value", false, map[string]interface{}{
			"key1": "value",
			"key2": []interface{}{"value"},
		}},

		{"separator in multiline", "key: >-\n  hello\n  ---\n  world", false, map[string]interface{}{
			"key": "hello --- world",
		}},

		{"multiple json", "{\"1\":2}{\"3\":4}", false, map[string]interface{}{
			"1": 2,
			"3": 4,
		}},
		{"json with spaces", "   {\"1\":2}    ", false, map[string]interface{}{
			"1": 2,
		}},

		{"big yaml", strings.Repeat("---\nkey1: value1\n---\nkey2: value2\n", 10*1024), false, map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}},
	}

	for _, testCase := range testCases {
		base := map[string]interface{}{}
		base, err := mergeYaml(base, []byte(testCase.input))

		switch {
		case testCase.err && err == nil:
			t.Errorf("%s: unexpected non-error", testCase.desc)
			continue
		case !testCase.err && err != nil:
			t.Errorf("%s: unexpected error: %v", testCase.desc, err)
			continue
		case err != nil:
			continue
		}

		if fmt.Sprintf("%#v", testCase.expect) != fmt.Sprintf("%#v", base) {
			t.Errorf("%s: objects were not equal: \n%#v\n%#v", testCase.desc, testCase.expect, base)
		}
	}
}
