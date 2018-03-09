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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/lint/support"
)

const templateTestBasedir = "./testdata/albatross"

func TestValidateAllowedExtension(t *testing.T) {
	var failTest = []string{"/foo", "/test.toml"}
	for _, test := range failTest {
		err := validateAllowedExtension(test)
		if err == nil || !strings.Contains(err.Error(), "Valid extensions are .yaml, .yml, .tpl, or .txt") {
			t.Errorf("validateAllowedExtension('%s') to return \"Valid extensions are .yaml, .yml, .tpl, or .txt\", got no error", test)
		}
	}
	var successTest = []string{"/foo.yaml", "foo.yaml", "foo.tpl", "/foo/bar/baz.yaml", "NOTES.txt"}
	for _, test := range successTest {
		err := validateAllowedExtension(test)
		if err != nil {
			t.Errorf("validateAllowedExtension('%s') to return no error but got \"%s\"", test, err.Error())
		}
	}
}

var values = []byte("nameOverride: ''\nhttpPort: 80")

const namespace = "testNamespace"
const strict = false

func TestTemplateParsing(t *testing.T) {
	linter := support.Linter{ChartDir: templateTestBasedir}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	if len(res) != 1 {
		t.Fatalf("Expected one error, got %d, %v", len(res), res)
	}

	if !strings.Contains(res[0].Err.Error(), "deliberateSyntaxError") {
		t.Errorf("Unexpected error: %s", res[0])
	}
}

var wrongTemplatePath = filepath.Join(templateTestBasedir, "templates", "fail.yaml")
var ignoredTemplatePath = filepath.Join(templateTestBasedir, "fail.yaml.ignored")

// Test a template with all the existing features:
// namespaces, partial templates
func TestTemplateIntegrationHappyPath(t *testing.T) {
	// Rename file so it gets ignored by the linter
	os.Rename(wrongTemplatePath, ignoredTemplatePath)
	defer os.Rename(ignoredTemplatePath, wrongTemplatePath)

	linter := support.Linter{ChartDir: templateTestBasedir}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	if len(res) != 0 {
		t.Fatalf("Expected no error, got %d, %v", len(res), res)
	}
}
