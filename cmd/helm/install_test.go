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

package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInstall(t *testing.T) {
	tests := []releaseCase{
		// Install, base case
		{
			name:     "basic install",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--name aeneas", " "),
			expected: "aeneas",
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		// Install, no hooks
		{
			name:     "install without hooks",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--name aeneas --no-hooks", " "),
			expected: "juno",
			resp:     releaseMock(&releaseOptions{name: "juno"}),
		},
		// Install, values from cli
		{
			name:     "install with values",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--set foo=bar", " "),
			resp:     releaseMock(&releaseOptions{name: "virgil"}),
			expected: "virgil",
		},
		// Install, no charts
		{
			name: "install with no chart specified",
			args: []string{},
			err:  true,
		},
		// Install, re-use name
		{
			name:     "install and replace release",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--name aeneas --replace", " "),
			expected: "aeneas",
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		// Install, using the name-template
		{
			name:     "install with name-template",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    []string{"--name-template", "{{upper \"foobar\"}}"},
			expected: "FOOBAR",
			resp:     releaseMock(&releaseOptions{name: "FOOBAR"}),
		},
		// Install, perform chart verification along the way.
		{
			name:  "install with verification, missing provenance",
			args:  []string{"testdata/testcharts/compressedchart-0.1.0.tgz"},
			flags: strings.Split("--verify --keyring testdata/helm-test-key.pub", " "),
			err:   true,
		},
		{
			name:  "install with verification, directory instead of file",
			args:  []string{"testdata/testcharts/signtest"},
			flags: strings.Split("--verify --keyring testdata/helm-test-key.pub", " "),
			err:   true,
		},
		{
			name:  "install with verification, valid",
			args:  []string{"testdata/testcharts/signtest-0.1.0.tgz"},
			flags: strings.Split("--verify --keyring testdata/helm-test-key.pub", " "),
		},
	}

	runReleaseCases(t, tests, func(c *fakeReleaseClient, out io.Writer) *cobra.Command {
		return newInstallCmd(c, out)
	})
}

func TestValues(t *testing.T) {
	args := "sailor=sinbad,good,port.source=baghdad,port.destination=basrah"
	vobj := new(values)
	vobj.Set(args)

	if vobj.Type() != "struct" {
		t.Fatalf("Expected Type to be struct, got %s", vobj.Type())
	}

	vals := vobj.pairs
	if fmt.Sprint(vals["good"]) != "true" {
		t.Errorf("Expected good to be true. Got %v", vals["good"])
	}

	port := vals["port"].(map[string]interface{})

	if fmt.Sprint(port["source"]) != "baghdad" {
		t.Errorf("Expected source to be baghdad. Got %s", port["source"])
	}
	if fmt.Sprint(port["destination"]) != "basrah" {
		t.Errorf("Expected source to be baghdad. Got %s", port["source"])
	}

	y := `good: true
port:
  destination: basrah
  source: baghdad
sailor: sinbad
`
	out, err := vobj.yaml()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != y {
		t.Errorf("Expected YAML to be \n%s\nGot\n%s\n", y, out)
	}

	if vobj.String() != y {
		t.Errorf("Expected String() to be \n%s\nGot\n%s\n", y, out)
	}

	// Combined case, overriding a property
	vals["sailor"] = "pisti"
	updatedYAML := `good: true
port:
  destination: basrah
  source: baghdad
sailor: pisti
`
	newOut, err := vobj.yaml()
	if err != nil {
		t.Fatal(err)
	}
	if string(newOut) != updatedYAML {
		t.Errorf("Expected YAML to be \n%s\nGot\n%s\n", updatedYAML, newOut)
	}

}

type nameTemplateTestCase struct {
	tpl              string
	expected         string
	expectedErrorStr string
}

func TestNameTemplate(t *testing.T) {
	testCases := []nameTemplateTestCase{
		// Just a straight up nop please
		{
			tpl:              "foobar",
			expected:         "foobar",
			expectedErrorStr: "",
		},
		// Random numbers at the end for fun & profit
		{
			tpl:              "foobar-{{randNumeric 6}}",
			expected:         "foobar-[0-9]{6}$",
			expectedErrorStr: "",
		},
		// Random numbers in the middle for fun & profit
		{
			tpl:              "foobar-{{randNumeric 4}}-baz",
			expected:         "foobar-[0-9]{4}-baz$",
			expectedErrorStr: "",
		},
		// No such function
		{
			tpl:              "foobar-{{randInt}}",
			expected:         "",
			expectedErrorStr: "function \"randInt\" not defined",
		},
		// Invalid template
		{
			tpl:              "foobar-{{",
			expected:         "",
			expectedErrorStr: "unexpected unclosed action",
		},
	}

	for _, tc := range testCases {

		n, err := generateName(tc.tpl)
		if err != nil {
			if tc.expectedErrorStr == "" {
				t.Errorf("Was not expecting error, but got: %v", err)
				continue
			}
			re, compErr := regexp.Compile(tc.expectedErrorStr)
			if compErr != nil {
				t.Errorf("Expected error string failed to compile: %v", compErr)
				continue
			}
			if !re.MatchString(err.Error()) {
				t.Errorf("Error didn't match for %s expected %s but got %v", tc.tpl, tc.expectedErrorStr, err)
				continue
			}
		}
		if err == nil && tc.expectedErrorStr != "" {
			t.Errorf("Was expecting error %s but didn't get an error back", tc.expectedErrorStr)
		}

		if tc.expected != "" {
			re, err := regexp.Compile(tc.expected)
			if err != nil {
				t.Errorf("Expected string failed to compile: %v", err)
				continue
			}
			if !re.MatchString(n) {
				t.Errorf("Returned name didn't match for %s expected %s but got %s", tc.tpl, tc.expected, n)
			}
		}
	}
}
