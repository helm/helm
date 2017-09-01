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

package main

import (
	"bytes"
	"regexp"
	"testing"
)

type templateCase struct {
	name  string
	args  []string
	flags []string
	// expected and notExpected are strings to be matched. This supports regular expressions.
	expected    []string
	notExpected []string
	err         bool
}

func TestTemplate(t *testing.T) {
	testCases := []templateCase{
		{
			name: "template basic",
			args: []string{"testdata/testcharts/templatetest"},
			expected: []string{
				"name: defaultname",
				"Source: templatetest/templates/deployment.yaml",
				"Source: templatetest/templates/service.yaml",
			},
			notExpected: []string{
				"1. These are the notes",
				"merged values",
			},
		},
		{
			name: "template missing chart",
			args: []string{""},
			err:  true,
		},
		{
			name:     "template with valid value file",
			args:     []string{"testdata/testcharts/templatetest"},
			flags:    []string{"--values", "testdata/testcharts/templatetest/other_values.yaml"},
			expected: []string{"name: othername"},
		},
		{
			name:  "template with invalid value file",
			args:  []string{"testdata/testcharts/templatetest"},
			flags: []string{"--values", ""},
			err:   true,
		},
		{
			name:     "template set existing key",
			args:     []string{"testdata/testcharts/templatetest"},
			flags:    []string{"--set", "name=customname"},
			expected: []string{"name: customname"},
		},
		{
			name:        "template set non-existing key",
			args:        []string{"testdata/testcharts/templatetest"},
			flags:       []string{"--set", "invalid=customvalue"},
			notExpected: []string{"customvalue"},
		},
		{
			name:     "template include notes",
			args:     []string{"testdata/testcharts/templatetest"},
			flags:    []string{"--notes"},
			expected: []string{"1. These are the notes"},
		},
		{
			name:     "template verbose output",
			args:     []string{"testdata/testcharts/templatetest"},
			flags:    []string{"--verbose"},
			expected: []string{"merged values"},
		},
		{
			name:        "template render specific existing file",
			args:        []string{"testdata/testcharts/templatetest"},
			flags:       []string{"--execute", "templatetest/templates/deployment.yaml"},
			expected:    []string{"Source: templatetest/templates/deployment.yaml"},
			notExpected: []string{"Source: templatetest/templates/service.yaml"},
		},
		{
			name:        "template render specific non-existing file",
			args:        []string{"testdata/testcharts/templatetest"},
			flags:       []string{"--execute", "templatetest/templates/ingress.yaml"},
			notExpected: []string{"Source: templatetest/templates/ingress.yaml"},
		},
	}

	var buf bytes.Buffer
	for _, tc := range testCases {
		cmd := newTemplateCmd(&buf)
		cmd.ParseFlags(tc.flags)
		err := cmd.RunE(cmd, tc.args)
		if (err != nil) != tc.err {
			t.Errorf("%q. expected error, got '%v'", tc.name, err)
		}

		by := buf.Bytes()
		for _, exp := range tc.expected {
			re := regexp.MustCompile(exp)
			if !re.Match(by) {
				t.Errorf("%q. expected\n%q\ngot\n%q", tc.name, exp, buf.String())
			}
		}
		for _, exp := range tc.notExpected {
			re := regexp.MustCompile(exp)
			if re.Match(by) {
				t.Errorf("%q. not expected\n%q\nin\n%q", tc.name, exp, buf.String())
			}
		}
		buf.Reset()
	}
}
