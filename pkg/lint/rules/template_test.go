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

package rules

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint/support"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const (
	strict              = false
	namespace           = "testNamespace"
	templateTestBasedir = "./testdata/albatross"
)

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

// TestSTrictTemplatePrasingMapError is a regression test.
//
// The template engine should not produce an error when a map in values.yaml does
// not contain all possible keys.
//
// See https://github.com/helm/helm/issues/7483 (helm v3)
// and https://github.com/helm/helm/issues/6705 (helm v2)
func TestStrictTemplateParsingMapError(t *testing.T) {
	dir, err := ioutil.TempDir("", "helm-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var vals = []byte(`
mymap:
  key1: nestedValue
`)
	var manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myval1: {{default "val" .Values.mymap.key1 }}
  myval2: {{default "val" .Values.mymap.key2 }}
`
	ch := chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "regression.6705",
			ApiVersion: "v1",
			Version:    "0.1.0",
		},
		Templates: []*chart.Template{
			{
				Name: "templates/configmap.yaml",
				Data: []byte(manifest),
			},
		},
	}

	if err := chartutil.SaveDir(&ch, dir); err != nil {
		t.Fatal(err)
	}
	linter := &support.Linter{
		ChartDir: filepath.Join(dir, ch.Metadata.Name),
	}
	Templates(linter, vals, namespace, true)
	if len(linter.Messages) != 0 {
		t.Errorf("expected zero messages, got %d", len(linter.Messages))
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %q", i, msg)
		}
	}
}
