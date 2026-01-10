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

package engine

import (
	"math"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFuncs(t *testing.T) {
	//TODO write tests for failure cases
	tests := []struct {
		tpl, expect string
		vars        interface{}
	}{{
		tpl:    `{{ toYaml . }}`,
		expect: `foo: bar`,
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ toYamlPretty . }}`,
		expect: "baz:\n  - 1\n  - 2\n  - 3",
		vars:   map[string]interface{}{"baz": []int{1, 2, 3}},
	}, {
		tpl:    `{{ toToml . }}`,
		expect: "foo = \"bar\"\n",
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ fromToml . }}`,
		expect: "map[hello:world]",
		vars:   `hello = "world"`,
	}, {
		tpl:    `{{ fromToml . }}`,
		expect: "map[table:map[keyInTable:valueInTable subtable:map[keyInSubtable:valueInSubTable]]]",
		vars: `
[table]
keyInTable = "valueInTable"
[table.subtable]
keyInSubtable = "valueInSubTable"`,
	}, {
		tpl:    `{{ fromToml . }}`,
		expect: "map[tableArray:[map[keyInElement0:valueInElement0] map[keyInElement1:valueInElement1]]]",
		vars: `
[[tableArray]]
keyInElement0 = "valueInElement0"
[[tableArray]]
keyInElement1 = "valueInElement1"`,
	}, {
		tpl:    `{{ fromToml . }}`,
		expect: "map[Error:toml: line 1: unexpected EOF; expected key separator '=']",
		vars:   "one",
	}, {
		tpl:    `{{ toJson . }}`,
		expect: `{"foo":"bar"}`,
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ fromYaml . }}`,
		expect: "map[hello:world]",
		vars:   `hello: world`,
	}, {
		tpl:    `{{ fromYamlArray . }}`,
		expect: "[one 2 map[name:helm]]",
		vars:   "- one\n- 2\n- name: helm\n",
	}, {
		tpl:    `{{ fromYamlArray . }}`,
		expect: "[one 2 map[name:helm]]",
		vars:   `["one", 2, { "name": "helm" }]`,
	}, {
		// Regression for https://github.com/helm/helm/issues/2271
		tpl:    `{{ toToml . }}`,
		expect: "[mast]\n  sail = \"white\"\n",
		vars:   map[string]map[string]string{"mast": {"sail": "white"}},
	}, {
		tpl:    `{{ fromYaml . }}`,
		expect: "map[Error:error unmarshaling JSON: while decoding JSON: json: cannot unmarshal array into Go value of type map[string]interface {}]",
		vars:   "- one\n- two\n",
	}, {
		tpl:    `{{ fromJson .}}`,
		expect: `map[hello:world]`,
		vars:   `{"hello":"world"}`,
	}, {
		tpl:    `{{ fromJson . }}`,
		expect: `map[Error:json: cannot unmarshal array into Go value of type map[string]interface {}]`,
		vars:   `["one", "two"]`,
	}, {
		tpl:    `{{ fromJsonArray . }}`,
		expect: `[one 2 map[name:helm]]`,
		vars:   `["one", 2, { "name": "helm" }]`,
	}, {
		tpl:    `{{ fromJsonArray . }}`,
		expect: `[json: cannot unmarshal object into Go value of type []interface {}]`,
		vars:   `{"hello": "world"}`,
	}, {
		tpl:    `{{ merge .dict (fromYaml .yaml) }}`,
		expect: `map[a:map[b:c]]`,
		vars:   map[string]interface{}{"dict": map[string]interface{}{"a": map[string]interface{}{"b": "c"}}, "yaml": `{"a":{"b":"d"}}`},
	}, {
		tpl:    `{{ merge (fromYaml .yaml) .dict }}`,
		expect: `map[a:map[b:d]]`,
		vars:   map[string]interface{}{"dict": map[string]interface{}{"a": map[string]interface{}{"b": "c"}}, "yaml": `{"a":{"b":"d"}}`},
	}, {
		tpl:    `{{ fromYaml . }}`,
		expect: `map[Error:error unmarshaling JSON: while decoding JSON: json: cannot unmarshal array into Go value of type map[string]interface {}]`,
		vars:   `["one", "two"]`,
	}, {
		tpl:    `{{ fromYamlArray . }}`,
		expect: `[error unmarshaling JSON: while decoding JSON: json: cannot unmarshal object into Go value of type []interface {}]`,
		vars:   `hello: world`,
	}, {
		// This should never result in a network lookup. Regression for #7955
		tpl:    `{{ lookup "v1" "Namespace" "" "unlikelynamespace99999999" }}`,
		expect: `map[]`,
		vars:   `["one", "two"]`,
	}}

	for _, tt := range tests {
		var b strings.Builder
		err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, tt.vars)
		assert.NoError(t, err)
		assert.Equal(t, tt.expect, b.String(), tt.tpl)
	}

	loopMap := map[string]interface{}{
		"foo": "bar",
	}
	loopMap["loop"] = []interface{}{loopMap}

	mustFuncsTests := []struct {
		tpl    string
		expect interface{}
		vars   interface{}
	}{{
		tpl:  `{{ mustToYaml . }}`,
		vars: loopMap,
	}, {
		tpl:  `{{ mustToJson . }}`,
		vars: loopMap,
	}, {
		tpl:    `{{ mustToDuration 30 }}`,
		expect: `30s`,
		vars:   nil,
	}, {
		tpl:    `{{ mustToDuration "1m30s" }}`,
		expect: `1m30s`,
		vars:   nil,
	}, {
		tpl:  `{{ mustToDuration "foo" }}`,
		vars: nil,
	}, {
		tpl:    `{{ toYaml . }}`,
		expect: "", // should return empty string and swallow error
		vars:   loopMap,
	}, {
		tpl:    `{{ toJson . }}`,
		expect: "", // should return empty string and swallow error
		vars:   loopMap,
	},
	}

	for _, tt := range mustFuncsTests {
		var b strings.Builder
		err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, tt.vars)
		if tt.expect != nil {
			assert.NoError(t, err)
			assert.Equal(t, tt.expect, b.String(), tt.tpl)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestDurationHelpers(t *testing.T) {
	tests := []struct {
		name   string
		tpl    string
		vars   interface{}
		expect string
	}{{
		name:   "toSeconds parses duration string",
		tpl:    `{{ toSeconds "1m30s" }}`,
		expect: `90`,
	}, {
		name:   "toSeconds parses numeric string as seconds",
		tpl:    `{{ toSeconds "2.5" }}`,
		expect: `2.5`,
	}, {
		name:   "toSeconds trims whitespace around numeric string",
		tpl:    `{{ toSeconds "  2.5  " }}`,
		expect: `2.5`,
	}, {
		name:   "toSeconds int treated as seconds",
		tpl:    `{{ toSeconds 2 }}`,
		expect: `2`,
	}, {
		name:   "toSeconds float treated as seconds",
		tpl:    `{{ toSeconds 2.5 }}`,
		expect: `2.5`,
	}, {
		name:   "toSeconds uint treated as seconds",
		tpl:    `{{ toSeconds . }}`,
		vars:   uint(2),
		expect: `2`,
	}, {
		name:   "toSeconds time.Duration passthrough",
		tpl:    `{{ toSeconds . }}`,
		vars:   1500 * time.Millisecond,
		expect: `1.5`,
	}, {
		name:   "invalid duration string returns 0",
		tpl:    `{{ toSeconds "nope" }}`,
		expect: `0`,
	}, {
		name:   "empty duration string returns 0",
		tpl:    `{{ toSeconds "" }}`,
		expect: `0`,
	}, {
		name:   "whitespace-only duration string returns 0",
		tpl:    `{{ toSeconds "   " }}`,
		expect: `0`,
	}, {
		name:   "nil returns 0",
		tpl:    `{{ toSeconds . }}`,
		vars:   nil,
		expect: `0`,
	}, {
		name:   "toSeconds uint overflow returns 0",
		tpl:    `{{ toSeconds . }}`,
		vars:   uint64(math.MaxInt64) + 1,
		expect: `0`,
	}, {
		name:   "toMilliseconds int seconds",
		tpl:    `{{ toMilliseconds 2 }}`,
		expect: `2000`,
	}, {
		name:   "toMilliseconds float seconds",
		tpl:    `{{ toMilliseconds 1.5 }}`,
		expect: `1500`,
	}, {
		name:   "toMicroseconds int seconds",
		tpl:    `{{ toMicroseconds 2 }}`,
		expect: `2000000`,
	}, {
		name:   "toNanoseconds int seconds",
		tpl:    `{{ toNanoseconds 2 }}`,
		expect: `2000000000`,
	}, {
		name:   "toMinutes parses duration string",
		tpl:    `{{ toMinutes "90s" }}`,
		expect: `1.5`,
	}, {
		name:   "toHours parses duration string",
		tpl:    `{{ toHours "90m" }}`,
		expect: `1.5`,
	}, {
		name:   "toDays parses duration string",
		tpl:    `{{ toDays "36h" }}`,
		expect: `1.5`,
	}, {
		name:   "toDays numeric seconds",
		tpl:    `{{ toDays 86400 }}`,
		expect: `1`,
	}, {
		name:   "roundToDuration numeric seconds",
		tpl:    `{{ roundToDuration 93 60 }}`, // 93s rounded to 60s = 120s
		expect: `2m0s`,
	}, {
		name:   "truncateToDuration numeric seconds",
		tpl:    `{{ truncateToDuration 93 60 }}`, // 93s truncated to 60s = 60s
		expect: `1m0s`,
	}, {
		name:   "roundToDuration accepts duration-string multiplier",
		tpl:    `{{ roundToDuration "93s" "1m" }}`,
		expect: `2m0s`,
	}, {
		name:   "truncateToDuration accepts duration-string multiplier",
		tpl:    `{{ truncateToDuration "93s" "1m" }}`,
		expect: `1m0s`,
	}, {
		name:   "roundToDuration invalid m returns v unchanged",
		tpl:    `{{ roundToDuration "93s" "nope" }}`,
		expect: `1m33s`,
	}, {
		name:   "truncateToDuration invalid m returns v unchanged",
		tpl:    `{{ truncateToDuration "93s" "nope" }}`,
		expect: `1m33s`,
	}, {
		name:   "roundToDuration zero m returns v unchanged",
		tpl:    `{{ roundToDuration "93s" 0 }}`,
		expect: `1m33s`,
	}, {
		name:   "truncateToDuration negative m returns v unchanged",
		tpl:    `{{ truncateToDuration "93s" -1 }}`,
		expect: `1m33s`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, tt.vars)
			assert.NoError(t, err, tt.tpl)
			assert.Equal(t, tt.expect, b.String(), tt.tpl)
		})
	}

	mustErrTests := []struct {
		name string
		tpl  string
		vars interface{}
	}{{
		name: "mustToDuration invalid string",
		tpl:  `{{ mustToDuration "nope" }}`,
	}, {
		name: "mustToDuration empty string",
		tpl:  `{{ mustToDuration "" }}`,
	}, {
		name: "mustToDuration whitespace string",
		tpl:  `{{ mustToDuration "   " }}`,
	}, {
		name: "mustToDuration unsupported type",
		tpl:  `{{ mustToDuration . }}`,
		vars: []int{1, 2, 3},
	}, {
		name: "mustToDuration uint overflow",
		tpl:  `{{ mustToDuration . }}`,
		vars: uint64(math.MaxInt64) + 1,
	},
	}

	for _, tt := range mustErrTests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, tt.vars)
			assert.Error(t, err, tt.tpl)
		})
	}
}

// This test to check a function provided by sprig is due to a change in a
// dependency of sprig. mergo in v0.3.9 changed the way it merges and only does
// public fields (i.e. those starting with a capital letter). This test, from
// sprig, fails in the new version. This is a behavior change for mergo that
// impacts sprig and Helm users. This test will help us to not update to a
// version of mergo (even accidentally) that causes a breaking change. See
// sprig changelog and notes for more details.
// Note, Go modules assume semver is never broken. So, there is no way to tell
// the tooling to not update to a minor or patch version. `go install` could
// be used to accidentally update mergo. This test and message should catch
// the problem and explain why it's happening.
func TestMerge(t *testing.T) {
	dict := map[string]interface{}{
		"src2": map[string]interface{}{
			"h": 10,
			"i": "i",
			"j": "j",
		},
		"src1": map[string]interface{}{
			"a": 1,
			"b": 2,
			"d": map[string]interface{}{
				"e": "four",
			},
			"g": []int{6, 7},
			"i": "aye",
			"j": "jay",
			"k": map[string]interface{}{
				"l": false,
			},
		},
		"dst": map[string]interface{}{
			"a": "one",
			"c": 3,
			"d": map[string]interface{}{
				"f": 5,
			},
			"g": []int{8, 9},
			"i": "eye",
			"k": map[string]interface{}{
				"l": true,
			},
		},
	}
	tpl := `{{merge .dst .src1 .src2}}`
	var b strings.Builder
	err := template.Must(template.New("test").Funcs(funcMap()).Parse(tpl)).Execute(&b, dict)
	assert.NoError(t, err)

	expected := map[string]interface{}{
		"a": "one", // key overridden
		"b": 2,     // merged from src1
		"c": 3,     // merged from dst
		"d": map[string]interface{}{ // deep merge
			"e": "four",
			"f": 5,
		},
		"g": []int{8, 9}, // overridden - arrays are not merged
		"h": 10,          // merged from src2
		"i": "eye",       // overridden twice
		"j": "jay",       // overridden and merged
		"k": map[string]interface{}{
			"l": true, // overridden
		},
	}
	assert.Equal(t, expected, dict["dst"])
}
