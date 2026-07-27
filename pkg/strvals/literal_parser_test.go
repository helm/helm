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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/yaml"
)

func TestParseLiteral(t *testing.T) {
	cases := []struct {
		str    string
		expect map[string]any
		err    bool
	}{
		{
			str: "name",
			err: true,
		},
		{
			str:    "name=",
			expect: map[string]any{"name": ""},
		},
		{
			str:    "name=value",
			expect: map[string]any{"name": "value"},
			err:    false,
		},
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
		{
			str:    "name1=null,name2=value2",
			expect: map[string]any{"name1": "null,name2=value2"},
			err:    false,
		},
		{
			str:    "name1=value,,,tail",
			expect: map[string]any{"name1": "value,,,tail"},
			err:    false,
		},
		{
			str:    "leading_zeros=00009",
			expect: map[string]any{"leading_zeros": "00009"},
			err:    false,
		},
		{
			str:    "name=one two three",
			expect: map[string]any{"name": "one two three"},
			err:    false,
		},
		{
			str:    "outer.inner=value",
			expect: map[string]any{"outer": map[string]any{"inner": "value"}},
			err:    false,
		},
		{
			str:    "outer.middle.inner=value",
			expect: map[string]any{"outer": map[string]any{"middle": map[string]any{"inner": "value"}}},
			err:    false,
		},
		{
			str: "name1.name2",
			err: true,
		},
		{
			str:    "name1.name2=",
			expect: map[string]any{"name1": map[string]any{"name2": ""}},
			err:    false,
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
			str:    "name1={value1,value2}",
			expect: map[string]any{"name1": "{value1,value2}"},
		},

		// List support
		{
			str:    "list[0]=foo",
			expect: map[string]any{"list": []string{"foo"}},
			err:    false,
		},
		{
			str: "list[0].foo=bar",
			expect: map[string]any{
				"list": []any{
					map[string]any{"foo": "bar"},
				},
			},
			err: false,
		},
		{
			str: "list[-30].hello=world",
			err: true,
		},
		{
			str:    "list[3]=bar",
			expect: map[string]any{"list": []any{nil, nil, nil, "bar"}},
			err:    false,
		},
		{
			str: "illegal[0]name.foo=bar",
			err: true,
		},
		{
			str:    "noval[0]",
			expect: map[string]any{"noval": []any{}},
			err:    false,
		},
		{
			str:    "noval[0]=",
			expect: map[string]any{"noval": []any{""}},
			err:    false,
		},
		{
			str:    "nested[0][0]=1",
			expect: map[string]any{"nested": []any{[]any{"1"}}},
			err:    false,
		},
		{
			str:    "nested[1][1]=1",
			expect: map[string]any{"nested": []any{nil, []any{nil, "1"}}},
			err:    false,
		},
		{
			str: "name1.name2[0].foo=bar",
			expect: map[string]any{
				"name1": map[string]any{
					"name2": []map[string]any{{"foo": "bar"}},
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
			str: "name1.name2[1].foo=bar",
			expect: map[string]any{
				"name1": map[string]any{
					"name2": []map[string]any{nil, {"foo": "bar"}},
				},
			},
		},
		{
			str:    "]={}].",
			expect: map[string]any{"]": "{}]."},
			err:    false,
		},

		// issue test cases: , = $ ( ) { } . \ \\
		{
			str:    "name=val,val",
			expect: map[string]any{"name": "val,val"},
			err:    false,
		},
		{
			str:    "name=val.val",
			expect: map[string]any{"name": "val.val"},
			err:    false,
		},
		{
			str:    "name=val=val",
			expect: map[string]any{"name": "val=val"},
			err:    false,
		},
		{
			str:    "name=val$val",
			expect: map[string]any{"name": "val$val"},
			err:    false,
		},
		{
			str:    "name=(value",
			expect: map[string]any{"name": "(value"},
			err:    false,
		},
		{
			str:    "name=value)",
			expect: map[string]any{"name": "value)"},
			err:    false,
		},
		{
			str:    "name=(value)",
			expect: map[string]any{"name": "(value)"},
			err:    false,
		},
		{
			str:    "name={value",
			expect: map[string]any{"name": "{value"},
			err:    false,
		},
		{
			str:    "name=value}",
			expect: map[string]any{"name": "value}"},
			err:    false,
		},
		{
			str:    "name={value}",
			expect: map[string]any{"name": "{value}"},
			err:    false,
		},
		{
			str:    "name={value1,value2}",
			expect: map[string]any{"name": "{value1,value2}"},
			err:    false,
		},
		{
			str:    `name=val\val`,
			expect: map[string]any{"name": `val\val`},
			err:    false,
		},
		{
			str:    `name=val\\val`,
			expect: map[string]any{"name": `val\\val`},
			err:    false,
		},
		{
			str:    `name=val\\\val`,
			expect: map[string]any{"name": `val\\\val`},
			err:    false,
		},
		{
			str:    `name={val,.?*v\0a!l)some`,
			expect: map[string]any{"name": `{val,.?*v\0a!l)some`},
			err:    false,
		},
		{
			str:    `name=em%GT)tqUDqz,i-\h+Mbqs-!:.m\\rE=mkbM#rR}@{-k@`,
			expect: map[string]any{"name": `em%GT)tqUDqz,i-\h+Mbqs-!:.m\\rE=mkbM#rR}@{-k@`},
		},
	}

	for _, tt := range cases {
		t.Run(tt.str, func(t *testing.T) {
			got, err := ParseLiteral(tt.str)
			if tt.err {
				require.Error(t, err)
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

func TestParseLiteralInto(t *testing.T) {
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
					"inner1": "value1,outer.inner3=value3,outer.inner4=4",
					"inner2": "value2",
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

	for _, tt := range tests {
		require.NoError(t, ParseLiteralInto(tt.input, tt.got))
		if tt.err {
			assert.Fail(t, "Expected error. Got nil", tt.input)
		}

		if tt.input2 != "" {
			require.NoError(t, ParseLiteralInto(tt.input2, tt.got))
			if tt.err {
				assert.Fail(t, "Expected error. Got nil", tt.input2)
			}
		}

		y1, err := yaml.Marshal(tt.expect)
		require.NoError(t, err)

		y2, err := yaml.Marshal(tt.got)
		require.NoError(t, err, "Error serializing parsed value")

		assert.YAMLEq(t, string(y1), string(y2), tt.input)
	}
}

func TestParseLiteralNestedLevels(t *testing.T) {
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
			str:    keyMultipleNestedLevels.String() + "=value",
			err:    true,
			errStr: fmt.Sprintf("value name nested level is greater than maximum supported nested level of %d", MaxNestedNameLevel),
		},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got, err := ParseLiteral(tt.str)
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
