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

package rules

import (
	"k8s.io/helm/pkg/lint/support"
	"strings"
	"testing"
)

const templateTestBasedir = "./testdata/albatross"

func TestValidateQuotes(t *testing.T) {
	// add `| quote` lint error
	var failTest = []string{"foo: {{.Release.Service }}", "foo:  {{.Release.Service }}", "- {{.Release.Service }}", "foo: {{default 'Never' .restart_policy}}", "-  {{.Release.Service }} "}

	for _, test := range failTest {
		err := validateQuotes("testTemplate.yaml", test)
		if err == nil || !strings.Contains(err.Error(), "add \"| quote\" to your substitution functions") {
			t.Errorf("validateQuotes('%s') to return \"add | quote error\", got no error", test)
		}
	}

	var successTest = []string{"foo: {{.Release.Service | quote }}", "foo:  {{.Release.Service | quote }}", "- {{.Release.Service | quote }}", "foo: {{default 'Never' .restart_policy | quote }}", "foo: \"{{ .Release.Service }}\"", "foo: \"{{ .Release.Service }} {{ .Foo.Bar }}\"", "foo: \"{{ default 'Never' .Release.Service }} {{ .Foo.Bar }}\""}

	for _, test := range successTest {
		err := validateQuotes("testTemplate.yaml", test)
		if err != nil {
			t.Errorf("validateQuotes('%s') to return not error and got \"%s\"", test, err.Error())
		}
	}

	// Surrounding quotes
	failTest = []string{"foo: {{.Release.Service }}-{{ .Release.Bar }}", "foo: {{.Release.Service }} {{ .Release.Bar }}", "- {{.Release.Service }}-{{ .Release.Bar }}", "- {{.Release.Service }}-{{ .Release.Bar }} {{ .Release.Baz }}", "foo: {{.Release.Service | default }}-{{ .Release.Bar }}"}

	for _, test := range failTest {
		err := validateQuotes("testTemplate.yaml", test)
		if err == nil || !strings.Contains(err.Error(), "wrap your substitution functions in double quotes") {
			t.Errorf("validateQuotes('%s') to return \"wrap your substitution functions in double quotes\", got no error %s", test, err.Error())
		}
	}

}

func TestTemplate(t *testing.T) {
	linter := support.Linter{ChartDir: templateTestBasedir}
	Templates(&linter)
	res := linter.Messages

	if len(res) != 1 {
		t.Fatalf("Expected one error, got %d, %v", len(res), res)
	}

	if !strings.Contains(res[0].Text, "deliberateSyntaxError") {
		t.Errorf("Unexpected error: %s", res[0])
	}
}
