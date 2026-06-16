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

package ignore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testdata = "./testdata"

func TestParse(t *testing.T) {
	rules := `#ignore

	#ignore
foo
bar/*
baz/bar/foo.txt

one/more
`
	r, err := parseString(rules)
	require.NoError(t, err, "Error parsing rules")

	assert.Len(t, r.patterns, 4)

	expects := []string{"foo", "bar/*", "baz/bar/foo.txt", "one/more"}
	for i, p := range r.patterns {
		assert.Equal(t, expects[i], p.raw)
		assert.NotNil(t, p.match, "Expected %s to have a matcher function.", p.raw)
	}
}

func TestParseFail(t *testing.T) {
	shouldFail := []string{"foo/**/bar", "[z-"}
	for _, fail := range shouldFail {
		_, err := parseString(fail)
		assert.Error(t, err, "Rule %q should have failed", fail)
	}
}

func TestParseFile(t *testing.T) {
	f := filepath.Join(testdata, HelmIgnore)
	_, err := os.Stat(f)
	require.NoError(t, err, "Fixture %s missing", f)

	r, err := ParseFile(f)
	require.NoError(t, err, "Failed to parse rules file")

	assert.Len(t, r.patterns, 3)
}

func TestIgnore(t *testing.T) {
	// Test table: Given pattern and name, Ignore should return expect.
	tests := []struct {
		pattern string
		name    string
		expect  bool
	}{
		// Glob tests
		{`helm.txt`, "helm.txt", true},
		{`helm.*`, "helm.txt", true},
		{`helm.*`, "rudder.txt", false},
		{`*.txt`, "tiller.txt", true},
		{`*.txt`, "cargo/a.txt", true},
		{`cargo/*.txt`, "cargo/a.txt", true},
		{`cargo/*.*`, "cargo/a.txt", true},
		{`cargo/*.txt`, "mast/a.txt", false},
		{`ru[c-e]?er.txt`, "rudder.txt", true},
		{`templates/.?*`, "templates/.dotfile", true},
		// "." should never get ignored. https://github.com/helm/helm/issues/1776
		{`.*`, ".", false},
		{`.*`, "./", false},
		{`.*`, ".joonix", true},
		{`.*`, "helm.txt", false},
		{`.*`, "", false},

		// Directory tests
		{`cargo/`, "cargo", true},
		{`cargo/`, "cargo/", true},
		{`cargo/`, "mast/", false},
		{`helm.txt/`, "helm.txt", false},

		// Negation tests
		{`!helm.txt`, "helm.txt", false},
		{`!helm.txt`, "tiller.txt", true},
		{`!*.txt`, "cargo", true},
		{`!cargo/`, "mast/", true},

		// Absolute path tests
		{`/a.txt`, "a.txt", true},
		{`/a.txt`, "cargo/a.txt", false},
		{`/cargo/a.txt`, "cargo/a.txt", true},
	}

	for _, test := range tests {
		r, err := parseString(test.pattern)
		require.NoError(t, err, "Failed to parse: %s", test.pattern)
		fi, err := os.Stat(filepath.Join(testdata, test.name))
		require.NoError(t, err, "Fixture missing: %s", test.name)

		assert.Equal(t, test.expect, r.Ignore(test.name, fi), "Expected %q to be %v for pattern %q", test.name, test.expect, test.pattern)
	}
}

func TestAddDefaults(t *testing.T) {
	r := Rules{}
	r.AddDefaults()

	assert.Len(t, r.patterns, 1)
}

func parseString(str string) (*Rules, error) {
	b := bytes.NewBuffer([]byte(str))
	return Parse(b)
}
