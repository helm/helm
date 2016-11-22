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
	"testing"

	"github.com/ghodss/yaml"
)

func TestParseSet(t *testing.T) {
	tests := []struct {
		str    string
		expect map[string]interface{}
	}{
		{
			"name1=value1",
			map[string]interface{}{"name1": "value1"},
		},
		{
			"name1=value1,name2=value2",
			map[string]interface{}{"name1": "value1", "name2": "value2"},
		},
		{
			"name1=value1,name2=value2,",
			map[string]interface{}{"name1": "value1", "name2": "value2"},
		},
		{
			"name1=value1,,,,name2=value2,",
			map[string]interface{}{"name1": "value1", "name2": "value2"},
		},
		{
			"name1=,name2=value2",
			map[string]interface{}{"name1": "", "name2": "value2"},
		},
		{
			"name1,name2=",
			map[string]interface{}{"name1": "", "name2": ""},
		},
		{
			"name1,name2=value2",
			map[string]interface{}{"name1": "", "name2": "value2"},
		},
		{
			"name1,name2=value2\\",
			map[string]interface{}{"name1": "", "name2": "value2"},
		},
		{
			"name1,name2",
			map[string]interface{}{"name1": "", "name2": ""},
		},
		{
			"name1=one\\,two,name2=three\\,four",
			map[string]interface{}{"name1": "one,two", "name2": "three,four"},
		},
		{
			"name1=one\\=two,name2=three\\=four",
			map[string]interface{}{"name1": "one=two", "name2": "three=four"},
		},
		{
			"name1=one two three,name2=three two one",
			map[string]interface{}{"name1": "one two three", "name2": "three two one"},
		},
		{
			"outer.inner=value",
			map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
		},
		{
			"outer.middle.inner=value",
			map[string]interface{}{"outer": map[string]interface{}{"middle": map[string]interface{}{"inner": "value"}}},
		},
		{
			"outer.inner1=value,outer.inner2=value2",
			map[string]interface{}{"outer": map[string]interface{}{"inner1": "value", "inner2": "value2"}},
		},
		{
			"outer.inner1=value,outer.middle.inner=value",
			map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": "value",
					"middle": map[string]interface{}{
						"inner": "value",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		got, err := Parse(tt.str)
		if err != nil {
			t.Fatal(err)
		}

		y1, err := yaml.Marshal(tt.expect)
		if err != nil {
			t.Fatal(err)
		}
		y2, err := yaml.Marshal(got)
		if err != nil {
			t.Fatalf("Error serializing parsed value: %s", err)
		}

		if string(y1) != string(y2) {
			t.Errorf("%s: Expected:\n%s\nGot:\n%s", tt.str, y1, y2)
		}
	}
}

func TestParseInto(t *testing.T) {
	got := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner1": "overwrite",
			"inner2": "value2",
		},
	}
	input := "outer.inner1=value1,outer.inner3=value3"
	expect := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner1": "value1",
			"inner2": "value2",
			"inner3": "value3",
		},
	}

	if err := ParseInto(input, got); err != nil {
		t.Fatal(err)
	}

	y1, err := yaml.Marshal(expect)
	if err != nil {
		t.Fatal(err)
	}
	y2, err := yaml.Marshal(got)
	if err != nil {
		t.Fatalf("Error serializing parsed value: %s", err)
	}

	if string(y1) != string(y2) {
		t.Errorf("%s: Expected:\n%s\nGot:\n%s", input, y1, y2)
	}
}

func TestToYAML(t *testing.T) {
	// The TestParse does the hard part. We just verify that YAML formatting is
	// happening.
	o, err := ToYAML("name=value")
	if err != nil {
		t.Fatal(err)
	}
	expect := "name: value\n"
	if o != expect {
		t.Errorf("Expected %q, got %q", expect, o)
	}
}
