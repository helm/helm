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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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
	nonListKeyMap := map[string]interface{}{
		"test": "fun",
	}
	stringListMap := map[string]interface{}{
		"test": []interface{}{
			"valueOne", "valueTwo", "valueThree",
		},
	}
	complexListMap := map[string]interface{}{
		"test": []interface{}{
			map[string]interface{}{
				"successful":           "value will be gone",
				"someKeyNotOverridden": "will also be gone",
			},
			map[string]interface{}{
				"someKey": "someValue",
			},
		},
	}
	ignoredStringListOverrideMap := map[string]interface{}{
		"test": []interface{}{
			"valueFour",
		},
		"test[1]": "valueFive",
	}
	invalidStringListOverride := map[string]interface{}{
		"test[7]": "invalidOverride",
	}
	invalidStringListFormat := map[string]interface{}{
		"test[]": "invalidOverride",
	}
	validStringListOverride := map[string]interface{}{
		"test[1]": "newValue",
	}

	validMapListOverride := map[string]interface{}{
		"test[0]": map[string]interface{}{
			"nested":     "values",
			"successful": "override",
		},
	}

	recursiveListMap := map[string]interface{}{
		"header": map[string]interface{}{
			"testing": []interface{}{
				map[string]interface{}{"override": "occurs"},
				"this one stays",
			},
		},
	}

	recursiveListOverride := map[string]interface{}{
		"header": map[string]interface{}{
			"testing[0]": "time to override",
		},
	}

	expectedRecursiveListOverride := map[string]interface{}{
		"header": map[string]interface{}{
			"testing": []interface{}{
				"time to override",
				"this one stays",
			},
		},
	}

	testMap, _ := mergeMaps(flatMap, nestedMap)
	equal := reflect.DeepEqual(testMap, nestedMap)
	if !equal {
		t.Errorf("Expected a nested map to overwrite a flat value. Expected: %v, got %v", nestedMap, testMap)
	}

	testMap, _ = mergeMaps(nestedMap, flatMap)
	equal = reflect.DeepEqual(testMap, flatMap)
	if !equal {
		t.Errorf("Expected a flat value to overwrite a map. Expected: %v, got %v", flatMap, testMap)
	}

	testMap, _ = mergeMaps(nestedMap, anotherNestedMap)
	equal = reflect.DeepEqual(testMap, anotherNestedMap)
	if !equal {
		t.Errorf("Expected a nested map to overwrite another nested map. Expected: %v, got %v", anotherNestedMap, testMap)
	}

	testMap, _ = mergeMaps(anotherFlatMap, anotherNestedMap)
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

	testMap, _ = mergeMaps(stringListMap, ignoredStringListOverrideMap)
	expectedMap = map[string]interface{}{
		"test": []interface{}{
			"valueFour",
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected an index key to be ignored when passed in with matching list key. Expected %v, got %v", expectedMap, testMap)
	}

	_, err := mergeMaps(stringListMap, invalidStringListOverride)
	if err == nil {
		t.Errorf("Expected error for invalid list override index")
	}
	assert.EqualError(t, err, "invalid key format test - index 7 does not exist in the destination list")

	_, err = mergeMaps(stringListMap, invalidStringListFormat)
	if err == nil {
		t.Errorf("Expected error for invalid key format")
	}
	assert.EqualError(t, err, "invalid key format test[] for list override - failed to find a valid index")

	_, err = mergeMaps(nonListKeyMap, validStringListOverride)
	if err == nil {
		t.Errorf("Expected error for invalid type of destination")
	}
	assert.EqualError(t, err, "invalid key test[1] - the underlying value in the base layer is not a list")

	testMap, _ = mergeMaps(stringListMap, validStringListOverride)
	expectedMap = map[string]interface{}{
		"test": []interface{}{
			"valueOne", "newValue", "valueThree",
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected index 1 to be overridden with string. Expected %v, got %v", expectedMap, testMap)
	}

	testMap, _ = mergeMaps(stringListMap, validMapListOverride)
	expectedMap = map[string]interface{}{
		"test": []interface{}{
			map[string]interface{}{
				"nested":     "values",
				"successful": "override",
			},
			"valueTwo",
			"valueThree",
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected index 0 to be overridden with map. Expected %v, got %v", expectedMap, testMap)
	}

	testMap, _ = mergeMaps(complexListMap, validMapListOverride)
	expectedMap = map[string]interface{}{
		"test": []interface{}{
			map[string]interface{}{
				// no merge of nested keys
				"nested":     "values",
				"successful": "override",
			},
			map[string]interface{}{
				"someKey": "someValue",
			},
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected map at index 0 to be overridden without merge. Expected %v, got %v", expectedMap, testMap)
	}

	testMap, _ = mergeMaps(recursiveListMap, recursiveListOverride)
	equal = reflect.DeepEqual(testMap, expectedRecursiveListOverride)
	if !equal {
		t.Errorf("Expected recursive list at index 0 to be overridden. Expected %v, got %v", expectedRecursiveListOverride, testMap)
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
