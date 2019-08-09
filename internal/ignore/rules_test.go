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
	if err != nil {
		t.Fatalf("Error parsing rules: %s", err)
	}

	if len(r.patterns) != 4 {
		t.Errorf("Expected 4 rules, got %d", len(r.patterns))
	}

	expects := []string{"foo", "bar/*", "baz/bar/foo.txt", "one/more"}
	for i, p := range r.patterns {
		if p.raw != expects[i] {
			t.Errorf("Expected %q, got %q", expects[i], p.raw)
		}
		if p.match == nil {
			t.Errorf("Expected %s to have a matcher function.", p.raw)
		}
	}
}

func TestParseFail(t *testing.T) {
	shouldFail := []string{"foo/**/bar", "[z-"}
	for _, fail := range shouldFail {
		_, err := parseString(fail)
		if err == nil {
			t.Errorf("Rule %q should have failed", fail)
		}
	}
}

func TestParseFile(t *testing.T) {
	f := filepath.Join(testdata, HelmIgnore)
	if _, err := os.Stat(f); err != nil {
		t.Fatalf("Fixture %s missing: %s", f, err)
	}

	r, err := ParseFile(f)
	if err != nil {
		t.Fatalf("Failed to parse rules file: %s", err)
	}

	if len(r.patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(r.patterns))
	}
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
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}
		fi, err := os.Stat(filepath.Join(testdata, test.name))
		if err != nil {
			t.Fatalf("Fixture missing: %s", err)
		}

		if r.Ignore(test.name, fi) != test.expect {
			t.Errorf("Expected %q to be %v for pattern %q", test.name, test.expect, test.pattern)
		}
	}
}

func TestAddDefaults(t *testing.T) {
	r := Rules{}
	r.AddDefaults()

	if len(r.patterns) != 1 {
		t.Errorf("Expected 1 default patterns, got %d", len(r.patterns))
	}
}

func parseString(str string) (*Rules, error) {
	b := bytes.NewBuffer([]byte(str))
	return Parse(b)
}
