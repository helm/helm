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

	"helm.sh/helm/v4/pkg/getter"
)

func TestReadFile(t *testing.T) {
	var p getter.Providers
	filePath := "%a.txt"
	_, err := readFile(filePath, p)
	if err == nil {
		t.Errorf("Expected error when has special strings")
	}
}

func TestMergeValues(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name: "set-json object",
			opts: Options{
				JSONValues: []string{`{"foo": {"bar": "baz"}}`},
			},
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		{
			name: "set-json key=value",
			opts: Options{
				JSONValues: []string{"foo.bar=[1,2,3]"},
			},
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": []interface{}{1.0, 2.0, 3.0},
				},
			},
		},
		{
			name: "set regular value",
			opts: Options{
				Values: []string{"foo=bar"},
			},
			expected: map[string]interface{}{
				"foo": "bar",
			},
		},
		{
			name: "set string value",
			opts: Options{
				StringValues: []string{"foo=123"},
			},
			expected: map[string]interface{}{
				"foo": "123",
			},
		},
		{
			name: "set literal value",
			opts: Options{
				LiteralValues: []string{"foo=true"},
			},
			expected: map[string]interface{}{
				"foo": "true",
			},
		},
		{
			name: "multiple options",
			opts: Options{
				Values:        []string{"a=foo"},
				StringValues:  []string{"b=bar"},
				JSONValues:    []string{`{"c": "foo1"}`},
				LiteralValues: []string{"d=bar1"},
			},
			expected: map[string]interface{}{
				"a": "foo",
				"b": "bar",
				"c": "foo1",
				"d": "bar1",
			},
		},
		{
			name: "invalid json",
			opts: Options{
				JSONValues: []string{`{invalid`},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.MergeValues(getter.Providers{})
			if (err != nil) != tt.wantErr {
				t.Errorf("MergeValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MergeValues() = %v, want %v", got, tt.expected)
			}
		})
	}
}
