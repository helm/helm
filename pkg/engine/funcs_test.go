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
)

func TestFuncs(t *testing.T) {
	//TODO write tests for failure cases
	tests := []struct {
		tpl, expect string
		vars        interface{}
	}{{
		tpl:    `All {{ required "A valid 'bases' is required" .bases }} of them!`,
		expect: `All 2 of them!`,
		vars:   map[string]interface{}{"bases": 2},
	}, {
		tpl:    `{{ toYaml . }}`,
		expect: `foo: bar`,
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ toToml . }}`,
		expect: "foo = \"bar\"\n",
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ toJson . }}`,
		expect: `{"foo":"bar"}`,
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ fromYaml . }}`,
		expect: "map[hello:world]",
		vars:   `hello: world`,
	}, {
		// Regression for https://github.com/helm/helm/issues/2271
		tpl:    `{{ toToml . }}`,
		expect: "[mast]\n  sail = \"white\"\n",
		vars:   map[string]map[string]string{"mast": {"sail": "white"}},
	}, {
		tpl:    `{{ fromYaml . }}`,
		expect: "map[Error:yaml: unmarshal errors:\n  line 1: cannot unmarshal !!seq into map[string]interface {}]",
		vars:   "- one\n- two\n",
	}, {
		tpl:    `{{ fromJson .}}`,
		expect: `map[hello:world]`,
		vars:   `{"hello":"world"}`,
	}, {
		tpl:    `{{ fromJson . }}`,
		expect: `map[Error:json: cannot unmarshal array into Go value of type map[string]interface {}]`,
		vars:   `["one", "two"]`,
	}}

	for _, tt := range tests {
		var b strings.Builder
		err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, tt.vars)
		assert.NoError(t, err)
		assert.Equal(t, tt.expect, b.String(), tt.tpl)
	}
}
