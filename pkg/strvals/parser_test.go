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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/yaml"
)

func TestSetIndex(t *testing.T) {
	tests := []struct {
		name    string
		initial []any
		expect  []any
		add     int
		val     int
		err     bool
	}{
		{
			name:    "short",
			initial: []any{0, 1},
			expect:  []any{0, 1, 2},
			add:     2,
			val:     2,
			err:     false,
		},
		{
			name:    "equal",
			initial: []any{0, 1},
			expect:  []any{0, 2},
			add:     1,
			val:     2,
			err:     false,
		},
		{
			name:    "long",
			initial: []any{0, 1, 2, 3, 4, 5},
			expect:  []any{0, 1, 2, 4, 4, 5},
			add:     3,
			val:     4,
			err:     false,
		},
		{
			name:    "negative",
			initial: []any{0, 1, 2, 3, 4, 5},
			expect:  []any{0, 1, 2, 3, 4, 5},
			add:     -1,
			val:     4,
			err:     true,
		},
		{
			name:    "large",
			initial: []any{0, 1, 2, 3, 4, 5},
			expect:  []any{0, 1, 2, 3, 4, 5},
			add:     MaxIndex + 1,
			val:     4,
			err:     true,
		},
	}

	for _, tt := range tests {
		got, err := setIndex(tt.initial, tt.add, tt.val)

		if err != nil && tt.err == false {
			require.Fail(t, "Expected no error but error returned", tt.name)
		} else if err == nil && tt.err == true {
			require.Fail(t, "Expected error but no error returned", tt.name)
		}

		if len(got) != len(tt.expect) {
			require.Len(t, got, len(tt.expect), tt.name)
		}

		if !tt.err {
			if gg := got[tt.add].(int); gg != tt.val {
				assert.Equal(t, tt.val, gg, tt.name)
			}
		}

		for k, v := range got {
			if v != tt.expect[k] {
				assert.Equal(t, tt.expect[k], v, tt.name)
			}
		}
	}
}

func TestParseSet(t *testing.T) {
	testsString := []struct {
		str    string
		expect map[string]any
		err    bool
	}{
		{
			str:    "long_int_string=1234567890",
			expect: map[string]any{"long_int_string": "1234567890"},
			err:    false,
		},
		{
			str:    "boolean=true",
			expect: map[string]any{"boolean": "true"},
			err:    false,
		},
		{
			str:    "is_null=null",
			expect: map[string]any{"is_null": "null"},
			err:    false,
		},
		{
			str:    "zero=0",
			expect: map[string]any{"zero": "0"},
			err:    false,
		},
	}
	tests := []struct {
		str    string
		expect map[string]any
		err    bool
	}{
		{
			"name1=null,f=false,t=true",
			map[string]any{"name1": nil, "f": false, "t": true},
			false,
		},
		{
			"name1=value1",
			map[string]any{"name1": "value1"},
			false,
		},
		{
			"name1=value1,name2=value2",
			map[string]any{"name1": "value1", "name2": "value2"},
			false,
		},
		{
			"name1=value1,name2=value2,",
			map[string]any{"name1": "value1", "name2": "value2"},
			false,
		},
		{
			str: "name1=value1,,,,name2=value2,",
			err: true,
		},
		{
			str:    "name1=,name2=value2",
			expect: map[string]any{"name1": "", "name2": "value2"},
		},
		{
			str:    "leading_zeros=00009",
			expect: map[string]any{"leading_zeros": "00009"},
		},
		{
			str:    "zero_int=0",
			expect: map[string]any{"zero_int": 0},
		},
		{
			str:    "long_int=1234567890",
			expect: map[string]any{"long_int": 1234567890},
		},
		{
			str:    "boolean=true",
			expect: map[string]any{"boolean": true},
		},
		{
			str:    "is_null=null",
			expect: map[string]any{"is_null": nil},
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
			map[string]any{"name1": "one,two", "name2": "three,four"},
			false,
		},
		{
			"name1=one\\=two,name2=three\\=four",
			map[string]any{"name1": "one=two", "name2": "three=four"},
			false,
		},
		{
			"name1=one two three,name2=three two one",
			map[string]any{"name1": "one two three", "name2": "three two one"},
			false,
		},
		{
			"outer.inner=value",
			map[string]any{"outer": map[string]any{"inner": "value"}},
			false,
		},
		{
			"outer.middle.inner=value",
			map[string]any{"outer": map[string]any{"middle": map[string]any{"inner": "value"}}},
			false,
		},
		{
			"outer.inner1=value,outer.inner2=value2",
			map[string]any{"outer": map[string]any{"inner1": "value", "inner2": "value2"}},
			false,
		},
		{
			"outer.inner1=value,outer.middle.inner=value",
			map[string]any{
				"outer": map[string]any{
					"inner1": "value",
					"middle": map[string]any{
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
			expect: map[string]any{"name1": map[string]any{"name2": ""}},
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
			map[string]any{"name1": []string{"value1", "value2"}},
			false,
		},
		{
			"name1={value1,value2},name2={value1,value2}",
			map[string]any{
				"name1": []string{"value1", "value2"},
				"name2": []string{"value1", "value2"},
			},
			false,
		},
		{
			"name1={1021,902}",
			map[string]any{"name1": []int{1021, 902}},
			false,
		},
		{
			"name1.name2={value1,value2}",
			map[string]any{"name1": map[string]any{"name2": []string{"value1", "value2"}}},
			false,
		},
		{
			str: "name1={1021,902",
			err: true,
		},
		// List support
		{
			str:    "list[0]=foo",
			expect: map[string]any{"list": []string{"foo"}},
		},
		{
			str: "list[0].foo=bar",
			expect: map[string]any{
				"list": []any{
					map[string]any{"foo": "bar"},
				},
			},
		},
		{
			str: "list[0].foo=bar,list[0].hello=world",
			expect: map[string]any{
				"list": []any{
					map[string]any{"foo": "bar", "hello": "world"},
				},
			},
		},
		{
			str: "list[0].foo=bar,list[-30].hello=world",
			err: true,
		},
		{
			str:    "list[0]=foo,list[1]=bar",
			expect: map[string]any{"list": []string{"foo", "bar"}},
		},
		{
			str:    "list[0]=foo,list[1]=bar,",
			expect: map[string]any{"list": []string{"foo", "bar"}},
		},
		{
			str:    "list[0]=foo,list[3]=bar",
			expect: map[string]any{"list": []any{"foo", nil, nil, "bar"}},
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
			expect: map[string]any{"noval": []any{}},
		},
		{
			str:    "noval[0]=",
			expect: map[string]any{"noval": []any{""}},
		},
		{
			str:    "nested[0][0]=1",
			expect: map[string]any{"nested": []any{[]any{1}}},
		},
		{
			str:    "nested[1][1]=1",
			expect: map[string]any{"nested": []any{nil, []any{nil, 1}}},
		},
		{
			str: "name1.name2[0].foo=bar,name1.name2[1].foo=bar",
			expect: map[string]any{
				"name1": map[string]any{
					"name2": []map[string]any{{"foo": "bar"}, {"foo": "bar"}},
				},
			},
		},
		{
			str: "name1.name2[1].foo=bar,name1.name2[0].foo=bar",
			expect: map[string]any{
				"name1": map[string]any{
					"name2": []map[string]any{{"foo": "bar"}, {"foo": "bar"}},
				},
			},
		},
		{
			str: "name1.name2[1].foo=bar",
			expect: map[string]any{
				"name1": map[string]any{
					"name2": []map[string]any{nil, {"foo": "bar"}},
				},
			},
		},
		{
			str: "]={}].",
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got, err := Parse(tt.str)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				y1, err := yaml.Marshal(tt.expect)
				require.NoError(t, err)
				y2, err := yaml.Marshal(got)
				if err != nil {
					require.NoError(t, err, "Error serializing parsed value")
				}

				assert.YAMLEq(t, string(y1), string(y2), tt.str)
			}
		})
	}
	for _, tt := range testsString {
		t.Run(tt.str, func(t *testing.T) {
			got, err := ParseString(tt.str)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				y1, err := yaml.Marshal(tt.expect)
				require.NoError(t, err)
				y2, err := yaml.Marshal(got)
				require.NoError(t, err, "Error serializing parsed value")
				assert.YAMLEq(t, string(y1), string(y2))
			}
		})
	}
}

func TestParseInto(t *testing.T) {
	tests := []struct {
		input  string
		input2 string
		got    map[string]any
		expect map[string]any
		err    bool
	}{
		{
			input: "outer.inner1=value1,outer.inner3=value3,outer.inner4=4",
			got: map[string]any{
				"outer": map[string]any{
					"inner1": "overwrite",
					"inner2": "value2",
				},
			},
			expect: map[string]any{
				"outer": map[string]any{
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
			got:    map[string]any{},
			expect: map[string]any{
				"listOuter": [][]any{{map[string]string{
					"type":   "listValue",
					"status": "alive",
				}}},
			},
			err: false,
		},
		{
			input:  "listOuter[0][0].type=listValue",
			input2: "listOuter[1][0].status=alive",
			got:    map[string]any{},
			expect: map[string]any{
				"listOuter": [][]any{
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
			got: map[string]any{
				"listOuter": []any{
					[]any{
						[]any{
							map[string]string{"exited": "old"},
						},
					},
				},
			},
			expect: map[string]any{
				"listOuter": [][][]any{
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
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			require.NoError(t, ParseInto(tt.input, tt.got))
			if tt.err {
				assert.Fail(t, "Expected error. Got nil", tt.input)
			}

			if tt.input2 != "" {
				require.NoError(t, ParseInto(tt.input2, tt.got))
				if tt.err {
					assert.Fail(t, "Expected error. Got nil", tt.input2)
				}
			}

			y1, err := yaml.Marshal(tt.expect)
			require.NoError(t, err)
			y2, err := yaml.Marshal(tt.got)
			require.NoError(t, err, "Error serializing parsed value")
			assert.YAMLEq(t, string(y1), string(y2), tt.input)
		})
	}
}

func TestParseIntoString(t *testing.T) {
	got := map[string]any{
		"outer": map[string]any{
			"inner1": "overwrite",
			"inner2": "value2",
		},
	}
	input := "outer.inner1=1,outer.inner3=3"
	expect := map[string]any{
		"outer": map[string]any{
			"inner1": "1",
			"inner2": "value2",
			"inner3": "3",
		},
	}

	if err := ParseIntoString(input, got); err != nil {
		require.NoError(t, err)
	}

	y1, err := yaml.Marshal(expect)
	require.NoError(t, err)
	y2, err := yaml.Marshal(got)
	require.NoError(t, err, "Error serializing parsed value")
	assert.YAMLEq(t, string(y1), string(y2), input)
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		input  string
		got    map[string]any
		expect map[string]any
		err    bool
	}{
		{ // set json scalars values, and replace one existing key
			input: "outer.inner1=\"1\",outer.inner3=3,outer.inner4=true,outer.inner5=\"true\"",
			got: map[string]any{
				"outer": map[string]any{
					"inner1": "overwrite",
					"inner2": "value2",
				},
			},
			expect: map[string]any{
				"outer": map[string]any{
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
			got: map[string]any{
				"outer": map[string]any{
					"inner1": map[string]any{
						"x": "overwrite",
					},
					"inner2": "value2",
					"inner3": []any{
						"overwrite",
					},
				},
			},
			expect: map[string]any{
				"outer": map[string]any{
					"inner1": map[string]any{"a": "1", "b": 2, "c": []any{1, 2, 3}},
					"inner2": "value2",
					"inner3": []any{"new value 1", "new value 2"},
					"inner4": map[string]any{"aa": "1", "bb": 2, "cc": []any{1, 2, 3}},
					"inner5": []any{map[string]any{"A": "1", "B": 2, "C": []any{1, 2, 3}}},
				},
			},
			err: false,
		},
		{ // null assignment, and no value assigned (equivalent to null)
			input: "outer.inner1=,outer.inner3={\"aa\":\"1\",\"bb\":2,\"cc\":[1,2,3]},outer.inner3.cc[1]=null",
			got: map[string]any{
				"outer": map[string]any{
					"inner1": map[string]any{
						"x": "overwrite",
					},
					"inner2": "value2",
				},
			},
			expect: map[string]any{
				"outer": map[string]any{
					"inner1": nil,
					"inner2": "value2",
					"inner3": map[string]any{"aa": "1", "bb": 2, "cc": []any{1, nil, 3}},
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
		t.Run(tt.input, func(t *testing.T) {
			err := ParseJSON(tt.input, tt.got)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				y1, err := yaml.Marshal(tt.expect)
				require.NoError(t, err, "Error serializing expected value")
				y2, err := yaml.Marshal(tt.got)
				require.NoError(t, err, "Error serializing parsed value")
				assert.YAMLEq(t, string(y1), string(y2))
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	input := "name1=path1"
	expect := map[string]any{
		"name1": "value1",
	}
	rs2v := func(rs []rune) (any, error) {
		v := string(rs)
		if v != "path1" {
			assert.Equal(t, "path1", v, "%s: runesToVal", input)
			return "", nil
		}
		return "value1", nil
	}

	got, err := ParseFile(input, rs2v)
	require.NoError(t, err)

	y1, err := yaml.Marshal(expect)
	require.NoError(t, err)

	y2, err := yaml.Marshal(got)
	require.NoError(t, err, "Error serializing parsed value")
	assert.YAMLEq(t, string(y1), string(y2), input)
}

func TestParseIntoFile(t *testing.T) {
	got := map[string]any{}
	input := "name1=path1"
	expect := map[string]any{
		"name1": "value1",
	}
	rs2v := func(rs []rune) (any, error) {
		v := string(rs)
		if v != "path1" {
			assert.Equal(t, "path1", v, "%s: runesToVal", input)
			return "", nil
		}
		return "value1", nil
	}

	if err := ParseIntoFile(input, got, rs2v); err != nil {
		require.NoError(t, err)
	}

	y1, err := yaml.Marshal(expect)
	require.NoError(t, err)
	y2, err := yaml.Marshal(got)
	require.NoError(t, err, "Error serializing parsed value")

	assert.YAMLEq(t, string(y1), string(y2), input)
}

func TestToYAML(t *testing.T) {
	// The TestParse does the hard part. We just verify that YAML formatting is
	// happening.
	o, err := ToYAML("name=value")
	require.NoError(t, err)
	expect := "name: value"
	if o != expect {
		assert.Equal(t, expect, o)
	}
}

func TestParseSetNestedLevels(t *testing.T) {
	var keyMultipleNestedLevels strings.Builder
	for i := 1; i <= MaxNestedNameLevel+2; i++ {
		tmpStr := fmt.Sprintf("name%d", i)
		if i <= MaxNestedNameLevel+1 {
			tmpStr += "."
		}
		keyMultipleNestedLevels.WriteString(tmpStr)
	}
	tests := []struct {
		str    string
		expect map[string]any
		err    bool
		errStr string
	}{
		{
			"outer.middle.inner=value",
			map[string]any{"outer": map[string]any{"middle": map[string]any{"inner": "value"}}},
			false,
			"",
		},
		{
			str: keyMultipleNestedLevels.String() + "=value",
			err: true,
			errStr: fmt.Sprintf("value name nested level is greater than maximum supported nested level of %d",
				MaxNestedNameLevel),
		},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got, err := Parse(tt.str)
			if tt.err {
				require.Error(t, err)
				if tt.errStr != "" {
					require.EqualError(t, err, tt.errStr)
				}
			} else {
				require.NoError(t, err)

				y1, err := yaml.Marshal(tt.expect)
				require.NoError(t, err)

				y2, err := yaml.Marshal(got)
				require.NoError(t, err, "Error serializing parsed value")
				assert.YAMLEq(t, string(y1), string(y2), tt.str)
			}
		})
	}
}
