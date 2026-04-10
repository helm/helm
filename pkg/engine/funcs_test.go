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
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestFuncs(t *testing.T) {
	//TODO write tests for failure cases
	tests := []struct {
		tpl, expect string
		vars        any
	}{{
		tpl:    `{{ toYaml . }}`,
		expect: `foo: bar`,
		vars:   map[string]any{"foo": "bar"},
	}, {
		tpl:    `{{ toYamlPretty . }}`,
		expect: "baz:\n  - 1\n  - 2\n  - 3",
		vars:   map[string]any{"baz": []int{1, 2, 3}},
	}, {
		tpl:    `{{ toToml . }}`,
		expect: "foo = \"bar\"\n",
		vars:   map[string]any{"foo": "bar"},
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
		vars:   map[string]any{"foo": "bar"},
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
		vars:   map[string]any{"dict": map[string]any{"a": map[string]any{"b": "c"}}, "yaml": `{"a":{"b":"d"}}`},
	}, {
		tpl:    `{{ merge (fromYaml .yaml) .dict }}`,
		expect: `map[a:map[b:d]]`,
		vars:   map[string]any{"dict": map[string]any{"a": map[string]any{"b": "c"}}, "yaml": `{"a":{"b":"d"}}`},
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

	loopMap := map[string]any{
		"foo": "bar",
	}
	loopMap["loop"] = []any{loopMap}

	mustFuncsTests := []struct {
		tpl    string
		expect any
		vars   any
	}{{
		tpl:  `{{ mustToYaml . }}`,
		vars: loopMap,
	}, {
		tpl:  `{{ mustToJson . }}`,
		vars: loopMap,
	}, {
		tpl:    `{{ toYaml . }}`,
		expect: "", // should return empty string and swallow error
		vars:   loopMap,
	}, {
		tpl:    `{{ toJson . }}`,
		expect: "", // should return empty string and swallow error
		vars:   loopMap,
	}, {
		tpl:  `{{ mustToToml . }}`,
		vars: map[int]string{1: "one"}, // non-string key is invalid in TOML
	}, {
		tpl:    `{{ mustToToml . }}`,
		expect: "foo = \"bar\"\n", // should succeed and return TOML string
		vars:   map[string]string{"foo": "bar"},
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

func TestHtpasswd(t *testing.T) {
	tests := []struct {
		name        string
		tpl         string
		expect      string
		expectError string
		validate    func(t *testing.T, rendered string)
	}{
		{
			name: "defaults to bcrypt",
			tpl:  `{{ htpasswd "testuser" "testpassword" }}`,
			validate: func(t *testing.T, rendered string) {
				t.Helper()
				parts := strings.SplitN(rendered, ":", 2)
				require.Len(t, parts, 2)
				assert.Equal(t, "testuser", parts[0])
				assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(parts[1]), []byte("testpassword")))
			},
		},
		{
			name: "supports explicit bcrypt algorithm",
			tpl:  `{{ htpasswd "testuser" "testpassword" "bcrypt" }}`,
			validate: func(t *testing.T, rendered string) {
				t.Helper()
				parts := strings.SplitN(rendered, ":", 2)
				require.Len(t, parts, 2)
				assert.Equal(t, "testuser", parts[0])
				assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(parts[1]), []byte("testpassword")))
			},
		},
		{
			name:   "supports sha algorithm",
			tpl:    `{{ htpasswd "testuser" "testpassword" "sha" }}`,
			expect: `testuser:{SHA}i7YRj4/Wk1rQh2o740pxfTJwj/0=`,
		},
		{
			name:   "preserves invalid username behavior",
			tpl:    `{{ htpasswd "bad:user" "testpassword" }}`,
			expect: `invalid username: bad:user`,
		},
		{
			name:        "rejects unsupported algorithms",
			tpl:         `{{ htpasswd "testuser" "testpassword" "md5" }}`,
			expectError: `unsupported htpasswd hash algorithm "md5"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, nil)
			if tt.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, b.String())
				return
			}

			assert.Equal(t, tt.expect, b.String())
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
	dict := map[string]any{
		"src2": map[string]any{
			"h": 10,
			"i": "i",
			"j": "j",
		},
		"src1": map[string]any{
			"a": 1,
			"b": 2,
			"d": map[string]any{
				"e": "four",
			},
			"g": []int{6, 7},
			"i": "aye",
			"j": "jay",
			"k": map[string]any{
				"l": false,
			},
		},
		"dst": map[string]any{
			"a": "one",
			"c": 3,
			"d": map[string]any{
				"f": 5,
			},
			"g": []int{8, 9},
			"i": "eye",
			"k": map[string]any{
				"l": true,
			},
		},
	}
	tpl := `{{merge .dst .src1 .src2}}`
	var b strings.Builder
	err := template.Must(template.New("test").Funcs(funcMap()).Parse(tpl)).Execute(&b, dict)
	assert.NoError(t, err)

	expected := map[string]any{
		"a": "one", // key overridden
		"b": 2,     // merged from src1
		"c": 3,     // merged from dst
		"d": map[string]any{ // deep merge
			"e": "four",
			"f": 5,
		},
		"g": []int{8, 9}, // overridden - arrays are not merged
		"h": 10,          // merged from src2
		"i": "eye",       // overridden twice
		"j": "jay",       // overridden and merged
		"k": map[string]any{
			"l": true, // overridden
		},
	}
	assert.Equal(t, expected, dict["dst"])
}
