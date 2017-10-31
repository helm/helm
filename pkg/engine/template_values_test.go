/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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
	"fmt"
	"regexp"
	"testing"

	"k8s.io/helm/pkg/chartutil"
)

func expandValuesHelper(input string) (chartutil.Values, error) {
	var vals chartutil.Values
	vals = chartutil.FromYaml(input)
	if err, found := vals["Error"]; found {
		return nil, fmt.Errorf("Unexpected YAML parse failure: %s", err.(string))
	}
	expanded, err := New().ExpandValues(chartutil.Values{"Values": vals})
	if err != nil {
		return nil, fmt.Errorf("Expansion failed: %s", err.Error())
	}
	return expanded, nil
}

func checkFullExpansionHelper(input string, expected string) error {
	expanded, err := expandValuesHelper(input)
	if err != nil {
		return err
	}
	output := chartutil.ToYaml(expanded["Values"])
	if "\n"+output != expected {
		return fmt.Errorf("Unexpected result from ExpandValues().\nGot: %s\nExpected: %s", output, expected)
	}
	return nil
}

func checkSingleValue(input string, path string, expected string) error {
	expanded, err := expandValuesHelper(input)
	if err != nil {
		return err
	}
	val, err := expanded.PathValue("Values." + path)
	if err != nil {
		return err
	}
	if val != expected {
		return fmt.Errorf("Unexpected expansion result for %q.\nGot: %s\nExpected: %s", path, val, expected)
	}
	return nil
}

func TestTemplateValueExpansion(t *testing.T) {
	input := `
a:
  aa: '{{ tval "b.ba.baa" }}2'                    # 12
  ab: '{{ tval "a.aa" }}3'                        # 123
b:
  ba:
    baa: 1
  bb: '0{{ tval "a.ab" -}} 4 {{- tval "b.bc" }}'  # 012345
  bc: 5
  bd: '{{ index (tval "d") 1 }}'                  # L1
c:
  ca: '{{ substr 2 5 (tval "b.bb") }}'            # 234
  cb: '{{ .Values.b.ba.baa }}'                    # Can still access things this way (and get 1 here)
  cc: '{{ .Values.a.aa }}'                        # ... but there will be no recursion, so this will just be '{{ tval.b.ba }}2'
d:
- L0
- 'L{{ tval "b.ba.baa" }}'                        # L1
e:
  bool: true                                      # Will just be left alone
  float: 1.234								      #           "
`

	expected := `
a:
  aa: "12"
  ab: "123"
b:
  ba:
    baa: 1
  bb: "012345"
  bc: 5
  bd: L1
c:
  ca: "234"
  cb: "1"
  cc: '{{ tval "b.ba.baa" }}2'
d:
- L0
- L1
e:
  bool: true
  float: 1.234
`
	if err := checkFullExpansionHelper(input, expected); err != nil {
		t.Error(err)
	}

	input = `
a:
- aa:
    aaa: '{{ (tval "b").ba.baa }}'
    aab: '{{ tval "b.ba.bac" }}'
b:
  ba:
    baa: 1
    bab:
    - unused
    - left
    - right
    bac: 2
c:
  ca: >-
    {{
    print
    (index (tval "b.ba").bab (atoi (index (tval "a") 0).aa.aaa))
    (index (tval "d") (atoi (index (tval "a") 0).aa.aaa)).db.dba.dbaa
    }}
d:
- da:
  - unused
- db:
    dba:
      dbaa: '{{ index (tval "b.ba.bab") (atoi (index (tval "a") 0).aa.aab) }}'
`
	if err := checkSingleValue(input, "c.ca", "leftright"); err != nil {
		t.Error(err)
	}

	input = `
a:
  aa: cc
b:
  bb: '{{ (index (tval "c") (tval "a.aa")).x.xx }}'
c:
  cc:
    x:
      xx: '{{ tval "c.cd.x.xx" }}'
  cd:
    x:
      xx: good
`
	if err := checkSingleValue(input, "b.bb", "good"); err != nil {
		t.Error(err)
	}
}

func TestTemplateValueExpansionErrors(t *testing.T) {
	checkExpError := func(input string, errRegex string) {
		if _, err := expandValuesHelper(input); err == nil {
			t.Errorf("Expected error matching %q but expansion succeeded", errRegex)
		} else if !regexp.MustCompile("(?i)" + errRegex).MatchString(err.Error()) {
			t.Errorf("Expected error matching %q but got %q", errRegex, err.Error())
		}
	}

	inputYaml := "a: { b: '{{ tval \"a.b\" }}' }"
	checkExpError(inputYaml, `cyclic reference to "a.b"`)

	inputYaml = "a: { b: '{{ tval \"c.d\" }}' }\nc: { d: '{{ tval \"a.b\" }}' }"
	checkExpError(inputYaml, `cyclic reference`)
	inputYaml = "a: { b: '{{ tval \"c.d\" }}' }\nc: { d: '{{ tval \"e.f\" }}' }\ne: { f: '{{ tval \"g.h\" }}' }\ng: { h: '{{ tval \"a.b\" }}' }"
	checkExpError(inputYaml, `cyclic reference`)

	inputYaml = "a: { b: '{{ tval \"c.d\" }}' }"
	checkExpError(inputYaml, `value "c.d" does not exist`)

	inputYaml = "a: { b: '{{ tval \"c.d\" }}' }\nc: { d: '{{ invalid }}' }"
	checkExpError(inputYaml, `function "invalid" not defined`) // This one happens during parse, so we don't catch it so specifically
	checkExpError(inputYaml, `parse error in value "c.d"`)     // The (decorated) template will still be reported though

	inputYaml = "a: { b: '{{ tval \"c.d\" }}' }\nc: { d: '{{ substr \"bad arg\" 0 0 }}' }"
	checkExpError(inputYaml, `error expanding value "c.d"`)
}
