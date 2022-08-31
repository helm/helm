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
	"testing"

	"sigs.k8s.io/yaml"
)

func TestSetIndex(t *testing.T) {
	tests := []struct {
		name    string
		initial []interface{}
		expect  []interface{}
		add     int
		val     int
		err     bool
	}{
		{
			name:    "short",
			initial: []interface{}{0, 1},
			expect:  []interface{}{0, 1, 2},
			add:     2,
			val:     2,
			err:     false,
		},
		{
			name:    "equal",
			initial: []interface{}{0, 1},
			expect:  []interface{}{0, 2},
			add:     1,
			val:     2,
			err:     false,
		},
		{
			name:    "long",
			initial: []interface{}{0, 1, 2, 3, 4, 5},
			expect:  []interface{}{0, 1, 2, 4, 4, 5},
			add:     3,
			val:     4,
			err:     false,
		},
		{
			name:    "negative",
			initial: []interface{}{0, 1, 2, 3, 4, 5},
			expect:  []interface{}{0, 1, 2, 3, 4, 5},
			add:     -1,
			val:     4,
			err:     true,
		},
		{
			name:    "large",
			initial: []interface{}{0, 1, 2, 3, 4, 5},
			expect:  []interface{}{0, 1, 2, 3, 4, 5},
			add:     MaxIndex + 1,
			val:     4,
			err:     true,
		},
	}

	for _, tt := range tests {
		got, err := setIndex(tt.initial, tt.add, tt.val)

		if err != nil && tt.err == false {
			t.Fatalf("%s: Expected no error but error returned", tt.name)
		} else if err == nil && tt.err == true {
			t.Fatalf("%s: Expected error but no error returned", tt.name)
		}

		if len(got) != len(tt.expect) {
			t.Fatalf("%s: Expected length %d, got %d", tt.name, len(tt.expect), len(got))
		}

		if !tt.err {
			if gg := got[tt.add].(int); gg != tt.val {
				t.Errorf("%s, Expected value %d, got %d", tt.name, tt.val, gg)
			}
		}

		for k, v := range got {
			if v != tt.expect[k] {
				t.Errorf("%s, Expected value %d, got %d", tt.name, tt.expect[k], v)
			}
		}
	}
}

func TestParseSet(t *testing.T) {
	testsString := []struct {
		str    string
		expect map[string]interface{}
		err    bool
	}{
		{
			str:    "long_int_string=1234567890",
			expect: map[string]interface{}{"long_int_string": "1234567890"},
			err:    false,
		},
		{
			str:    "boolean=true",
			expect: map[string]interface{}{"boolean": "true"},
			err:    false,
		},
		{
			str:    "is_null=null",
			expect: map[string]interface{}{"is_null": "null"},
			err:    false,
		},
		{
			str:    "zero=0",
			expect: map[string]interface{}{"zero": "0"},
			err:    false,
		},
	}
	tests := []struct {
		str    string
		expect map[string]interface{}
		err    bool
	}{
		{
			"name1=null,f=false,t=true",
			map[string]interface{}{"name1": nil, "f": false, "t": true},
			false,
		},
		{
			"name1=value1",
			map[string]interface{}{"name1": "value1"},
			false,
		},
		{
			"name1=value1,name2=value2",
			map[string]interface{}{"name1": "value1", "name2": "value2"},
			false,
		},
		{
			"name1=value1,name2=value2,",
			map[string]interface{}{"name1": "value1", "name2": "value2"},
			false,
		},
		{
			str: "name1=value1,,,,name2=value2,",
			err: true,
		},
		{
			str:    "name1=,name2=value2",
			expect: map[string]interface{}{"name1": "", "name2": "value2"},
		},
		{
			str:    "leading_zeros=00009",
			expect: map[string]interface{}{"leading_zeros": "00009"},
		},
		{
			str:    "zero_int=0",
			expect: map[string]interface{}{"zero_int": 0},
		},
		{
			str:    "long_int=1234567890",
			expect: map[string]interface{}{"long_int": 1234567890},
		},
		{
			str:    "boolean=true",
			expect: map[string]interface{}{"boolean": true},
		},
		{
			str:    "is_null=null",
			expect: map[string]interface{}{"is_null": nil},
			err:    false,
		},
		{
			str: "name1,name2=",
			err: true,
		},
		{
			str: "name1,name2=value2",
			err: true,
		},
		{
			str: "name1,name2=value2\\",
			err: true,
		},
		{
			str: "name1,name2",
			err: true,
		},
		{
			"name1=one\\,two,name2=three\\,four",
			map[string]interface{}{"name1": "one,two", "name2": "three,four"},
			false,
		},
		{
			"name1=one\\=two,name2=three\\=four",
			map[string]interface{}{"name1": "one=two", "name2": "three=four"},
			false,
		},
		{
			"name1=one two three,name2=three two one",
			map[string]interface{}{"name1": "one two three", "name2": "three two one"},
			false,
		},
		{
			"outer.inner=value",
			map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
			false,
		},
		{
			"outer.middle.inner=value",
			map[string]interface{}{"outer": map[string]interface{}{"middle": map[string]interface{}{"inner": "value"}}},
			false,
		},
		{
			"outer.inner1=value,outer.inner2=value2",
			map[string]interface{}{"outer": map[string]interface{}{"inner1": "value", "inner2": "value2"}},
			false,
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
			false,
		},
		{
			str: "name1.name2",
			err: true,
		},
		{
			str: "name1.name2,name1.name3",
			err: true,
		},
		{
			str:    "name1.name2=",
			expect: map[string]interface{}{"name1": map[string]interface{}{"name2": ""}},
		},
		{
			str: "name1.=name2",
			err: true,
		},
		{
			str: "name1.,name2",
			err: true,
		},
		{
			"name1={value1,value2}",
			map[string]interface{}{"name1": []string{"value1", "value2"}},
			false,
		},
		{
			"name1={value1,value2},name2={value1,value2}",
			map[string]interface{}{
				"name1": []string{"value1", "value2"},
				"name2": []string{"value1", "value2"},
			},
			false,
		},
		{
			"name1={1021,902}",
			map[string]interface{}{"name1": []int{1021, 902}},
			false,
		},
		{
			"name1.name2={value1,value2}",
			map[string]interface{}{"name1": map[string]interface{}{"name2": []string{"value1", "value2"}}},
			false,
		},
		{
			str: "name1={1021,902",
			err: true,
		},
		// List support
		{
			str:    "list[0]=foo",
			expect: map[string]interface{}{"list": []string{"foo"}},
		},
		{
			str: "list[0].foo=bar",
			expect: map[string]interface{}{
				"list": []interface{}{
					map[string]interface{}{"foo": "bar"},
				},
			},
		},
		{
			str: "list[0].foo=bar,list[0].hello=world",
			expect: map[string]interface{}{
				"list": []interface{}{
					map[string]interface{}{"foo": "bar", "hello": "world"},
				},
			},
		},
		{
			str: "list[0].foo=bar,list[-30].hello=world",
			err: true,
		},
		{
			str:    "list[0]=foo,list[1]=bar",
			expect: map[string]interface{}{"list": []string{"foo", "bar"}},
		},
		{
			str:    "list[0]=foo,list[1]=bar,",
			expect: map[string]interface{}{"list": []string{"foo", "bar"}},
		},
		{
			str:    "list[0]=foo,list[3]=bar",
			expect: map[string]interface{}{"list": []interface{}{"foo", nil, nil, "bar"}},
		},
		{
			str: "list[0]=foo,list[-20]=bar",
			err: true,
		},
		{
			str: "illegal[0]name.foo=bar",
			err: true,
		},
		{
			str:    "noval[0]",
			expect: map[string]interface{}{"noval": []interface{}{}},
		},
		{
			str:    "noval[0]=",
			expect: map[string]interface{}{"noval": []interface{}{""}},
		},
		{
			str:    "nested[0][0]=1",
			expect: map[string]interface{}{"nested": []interface{}{[]interface{}{1}}},
		},
		{
			str:    "nested[1][1]=1",
			expect: map[string]interface{}{"nested": []interface{}{nil, []interface{}{nil, 1}}},
		},
		{
			str: "name1.name2[0].foo=bar,name1.name2[1].foo=bar",
			expect: map[string]interface{}{
				"name1": map[string]interface{}{
					"name2": []map[string]interface{}{{"foo": "bar"}, {"foo": "bar"}},
				},
			},
		},
		{
			str: "name1.name2[1].foo=bar,name1.name2[0].foo=bar",
			expect: map[string]interface{}{
				"name1": map[string]interface{}{
					"name2": []map[string]interface{}{{"foo": "bar"}, {"foo": "bar"}},
				},
			},
		},
		{
			str: "name1.name2[1].foo=bar",
			expect: map[string]interface{}{
				"name1": map[string]interface{}{
					"name2": []map[string]interface{}{nil, {"foo": "bar"}},
				},
			},
		},
		{
			str: "]={}].",
			err: true,
		},
	}

	for _, tt := range tests {
		got, err := Parse(tt.str)
		if err != nil {
			if tt.err {
				continue
			}
			t.Fatalf("%s: %s", tt.str, err)
		}
		if tt.err {
			t.Errorf("%s: Expected error. Got nil", tt.str)
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
	for _, tt := range testsString {
		got, err := ParseString(tt.str)
		if err != nil {
			if tt.err {
				continue
			}
			t.Fatalf("%s: %s", tt.str, err)
		}
		if tt.err {
			t.Errorf("%s: Expected error. Got nil", tt.str)
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
	tests := []struct {
		input  string
		input2 string
		got    map[string]interface{}
		expect map[string]interface{}
		err    bool
	}{
		{
			input: "outer.inner1=value1,outer.inner3=value3,outer.inner4=4",
			got: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": "overwrite",
					"inner2": "value2",
				},
			},
			expect: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": "value1",
					"inner2": "value2",
					"inner3": "value3",
					"inner4": 4,
				}},
			err: false,
		},
		{
			input:  "listOuter[0][0].type=listValue",
			input2: "listOuter[0][0].status=alive",
			got:    map[string]interface{}{},
			expect: map[string]interface{}{
				"listOuter": [][]interface{}{{map[string]string{
					"type":   "listValue",
					"status": "alive",
				}}},
			},
			err: false,
		},
		{
			input:  "listOuter[0][0].type=listValue",
			input2: "listOuter[1][0].status=alive",
			got:    map[string]interface{}{},
			expect: map[string]interface{}{
				"listOuter": [][]interface{}{
					{
						map[string]string{"type": "listValue"},
					},
					{
						map[string]string{"status": "alive"},
					},
				},
			},
			err: false,
		},
		{
			input:  "listOuter[0][1][0].type=listValue",
			input2: "listOuter[0][0][1].status=alive",
			got: map[string]interface{}{
				"listOuter": []interface{}{
					[]interface{}{
						[]interface{}{
							map[string]string{"exited": "old"},
						},
					},
				},
			},
			expect: map[string]interface{}{
				"listOuter": [][][]interface{}{
					{
						{
							map[string]string{"exited": "old"},
							map[string]string{"status": "alive"},
						},
						{
							map[string]string{"type": "listValue"},
						},
					},
				},
			},
			err: false,
		},
	}
	for _, tt := range tests {
		if err := ParseInto(tt.input, tt.got); err != nil {
			t.Fatal(err)
		}
		if tt.err {
			t.Errorf("%s: Expected error. Got nil", tt.input)
		}

		if tt.input2 != "" {
			if err := ParseInto(tt.input2, tt.got); err != nil {
				t.Fatal(err)
			}
			if tt.err {
				t.Errorf("%s: Expected error. Got nil", tt.input2)
			}
		}

		y1, err := yaml.Marshal(tt.expect)
		if err != nil {
			t.Fatal(err)
		}
		y2, err := yaml.Marshal(tt.got)
		if err != nil {
			t.Fatalf("Error serializing parsed value: %s", err)
		}

		if string(y1) != string(y2) {
			t.Errorf("%s: Expected:\n%s\nGot:\n%s", tt.input, y1, y2)
		}
	}
}

func TestParseIntoString(t *testing.T) {
	got := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner1": "overwrite",
			"inner2": "value2",
		},
	}
	input := "outer.inner1=1,outer.inner3=3"
	expect := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner1": "1",
			"inner2": "value2",
			"inner3": "3",
		},
	}

	if err := ParseIntoString(input, got); err != nil {
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

func TestParseJSON(t *testing.T) {
	tests := []struct {
		input  string
		got    map[string]interface{}
		expect map[string]interface{}
		err    bool
	}{
		{ // set json scalars values, and replace one existing key
			input: "outer.inner1=\"1\",outer.inner3=3,outer.inner4=true,outer.inner5=\"true\"",
			got: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": "overwrite",
					"inner2": "value2",
				},
			},
			expect: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": "1",
					"inner2": "value2",
					"inner3": 3,
					"inner4": true,
					"inner5": "true",
				},
			},
			err: false,
		},
		{ // set json objects and arrays, and replace one existing key
			input: "outer.inner1={\"a\":\"1\",\"b\":2,\"c\":[1,2,3]},outer.inner3=[\"new value 1\",\"new value 2\"],outer.inner4={\"aa\":\"1\",\"bb\":2,\"cc\":[1,2,3]},outer.inner5=[{\"A\":\"1\",\"B\":2,\"C\":[1,2,3]}]",
			got: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": map[string]interface{}{
						"x": "overwrite",
					},
					"inner2": "value2",
					"inner3": []interface{}{
						"overwrite",
					},
				},
			},
			expect: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": map[string]interface{}{"a": "1", "b": 2, "c": []interface{}{1, 2, 3}},
					"inner2": "value2",
					"inner3": []interface{}{"new value 1", "new value 2"},
					"inner4": map[string]interface{}{"aa": "1", "bb": 2, "cc": []interface{}{1, 2, 3}},
					"inner5": []interface{}{map[string]interface{}{"A": "1", "B": 2, "C": []interface{}{1, 2, 3}}},
				},
			},
			err: false,
		},
		{ // null assigment, and no value assigned (equivalent to null)
			input: "outer.inner1=,outer.inner3={\"aa\":\"1\",\"bb\":2,\"cc\":[1,2,3]},outer.inner3.cc[1]=null",
			got: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": map[string]interface{}{
						"x": "overwrite",
					},
					"inner2": "value2",
				},
			},
			expect: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner1": nil,
					"inner2": "value2",
					"inner3": map[string]interface{}{"aa": "1", "bb": 2, "cc": []interface{}{1, nil, 3}},
				},
			},
			err: false,
		},
		{ // syntax error
			input:  "outer.inner1={\"a\":\"1\",\"b\":2,\"c\":[1,2,3]},outer.inner3=[\"new value 1\",\"new value 2\"],outer.inner4={\"aa\":\"1\",\"bb\":2,\"cc\":[1,2,3]},outer.inner5={\"A\":\"1\",\"B\":2,\"C\":[1,2,3]}]",
			got:    nil,
			expect: nil,
			err:    true,
		},
	}
	for _, tt := range tests {
		if err := ParseJSON(tt.input, tt.got); err != nil {
			if tt.err {
				continue
			}
			t.Fatalf("%s: %s", tt.input, err)
		}
		if tt.err {
			t.Fatalf("%s: Expected error. Got nil", tt.input)
		}
		y1, err := yaml.Marshal(tt.expect)
		if err != nil {
			t.Fatalf("Error serializing expected value: %s", err)
		}
		y2, err := yaml.Marshal(tt.got)
		if err != nil {
			t.Fatalf("Error serializing parsed value: %s", err)
		}

		if string(y1) != string(y2) {
			t.Errorf("%s: Expected:\n%s\nGot:\n%s", tt.input, y1, y2)
		}
	}
}

func TestParseFile(t *testing.T) {
	input := "name1=path1"
	expect := map[string]interface{}{
		"name1": "value1",
	}
	rs2v := func(rs []rune) (interface{}, error) {
		v := string(rs)
		if v != "path1" {
			t.Errorf("%s: runesToVal: Expected value path1, got %s", input, v)
			return "", nil
		}
		return "value1", nil
	}

	got, err := ParseFile(input, rs2v)
	if err != nil {
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

func TestParseIntoFile(t *testing.T) {
	got := map[string]interface{}{}
	input := "name1=path1"
	expect := map[string]interface{}{
		"name1": "value1",
	}
	rs2v := func(rs []rune) (interface{}, error) {
		v := string(rs)
		if v != "path1" {
			t.Errorf("%s: runesToVal: Expected value path1, got %s", input, v)
			return "", nil
		}
		return "value1", nil
	}

	if err := ParseIntoFile(input, got, rs2v); err != nil {
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
	expect := "name: value"
	if o != expect {
		t.Errorf("Expected %q, got %q", expect, o)
	}
}
