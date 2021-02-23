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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/goutils"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/internal/third_party/dep/fs"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
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

var values = map[string]interface{}{"nameOverride": "", "httpPort": 80}

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
	fs.RenameWithFallback(wrongTemplatePath, ignoredTemplatePath)
	defer fs.RenameWithFallback(ignoredTemplatePath, wrongTemplatePath)

	linter := support.Linter{ChartDir: templateTestBasedir}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	if len(res) != 0 {
		t.Fatalf("Expected no error, got %d, %v", len(res), res)
	}
}

func TestV3Fail(t *testing.T) {
	linter := support.Linter{ChartDir: "./testdata/v3-fail"}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	if len(res) != 3 {
		t.Fatalf("Expected 3 errors, got %d, %v", len(res), res)
	}

	if !strings.Contains(res[0].Err.Error(), ".Release.Time has been removed in v3") {
		t.Errorf("Unexpected error: %s", res[0].Err)
	}
	if !strings.Contains(res[1].Err.Error(), "manifest is a crd-install hook") {
		t.Errorf("Unexpected error: %s", res[1].Err)
	}
	if !strings.Contains(res[2].Err.Error(), "manifest is a crd-install hook") {
		t.Errorf("Unexpected error: %s", res[2].Err)
	}
}

func TestMultiTemplateFail(t *testing.T) {
	linter := support.Linter{ChartDir: "./testdata/multi-template-fail"}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	if len(res) != 1 {
		t.Fatalf("Expected 1 error, got %d, %v", len(res), res)
	}

	if !strings.Contains(res[0].Err.Error(), "object name does not conform to Kubernetes naming requirements") {
		t.Errorf("Unexpected error: %s", res[0].Err)
	}
}

func TestValidateMetadataName(t *testing.T) {
	names := map[string]bool{
		"":                          false,
		"foo":                       true,
		"foo.bar1234baz.seventyone": true,
		"FOO":                       false,
		"123baz":                    true,
		"foo.BAR.baz":               false,
		"one-two":                   true,
		"-two":                      false,
		"one_two":                   false,
		"a..b":                      false,
		"%^&#$%*@^*@&#^":            false,
	}

	// The length checker should catch this first. So this is not true fuzzing.
	tooLong, err := goutils.RandomAlphaNumeric(300)
	if err != nil {
		t.Fatalf("Randomizer failed to initialize: %s", err)
	}
	names[tooLong] = false

	for input, expectPass := range names {
		obj := K8sYamlStruct{Metadata: k8sYamlMetadata{Name: input}}
		if err := validateMetadataName(&obj); (err == nil) != expectPass {
			st := "fail"
			if expectPass {
				st = "succeed"
			}
			t.Errorf("Expected %q to %s", input, st)
			if err != nil {
				t.Log(err)
			}
		}
	}
}

func TestDeprecatedAPIFails(t *testing.T) {
	mychart := chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v2",
			Name:       "failapi",
			Version:    "0.1.0",
			Icon:       "satisfy-the-linting-gods.gif",
		},
		Templates: []*chart.File{
			{
				Name: "templates/baddeployment.yaml",
				Data: []byte("apiVersion: apps/v1beta1\nkind: Deployment\nmetadata:\n  name: baddep\nspec: {selector: {matchLabels: {foo: bar}}}"),
			},
			{
				Name: "templates/goodsecret.yaml",
				Data: []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: goodsecret"),
			},
		},
	}
	tmpdir := ensure.TempDir(t)
	defer os.RemoveAll(tmpdir)

	if err := chartutil.SaveDir(&mychart, tmpdir); err != nil {
		t.Fatal(err)
	}

	linter := support.Linter{ChartDir: filepath.Join(tmpdir, mychart.Name())}
	Templates(&linter, values, namespace, strict)
	if l := len(linter.Messages); l != 1 {
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %s", i, msg)
		}
		t.Fatalf("Expected 1 lint error, got %d", l)
	}

	err := linter.Messages[0].Err.(deprecatedAPIError)
	if err.Deprecated != "apps/v1beta1 Deployment" {
		t.Errorf("Surprised to learn that %q is deprecated", err.Deprecated)
	}
}

const manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myval1: {{default "val" .Values.mymap.key1 }}
  myval2: {{default "val" .Values.mymap.key2 }}
`

// TestSTrictTemplatePrasingMapError is a regression test.
//
// The template engine should not produce an error when a map in values.yaml does
// not contain all possible keys.
//
// See https://github.com/helm/helm/issues/7483
func TestStrictTemplateParsingMapError(t *testing.T) {

	ch := chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "regression7483",
			APIVersion: "v2",
			Version:    "0.1.0",
		},
		Values: map[string]interface{}{
			"mymap": map[string]string{
				"key1": "val1",
			},
		},
		Templates: []*chart.File{
			{
				Name: "templates/configmap.yaml",
				Data: []byte(manifest),
			},
		},
	}
	dir := ensure.TempDir(t)
	defer os.RemoveAll(dir)
	if err := chartutil.SaveDir(&ch, dir); err != nil {
		t.Fatal(err)
	}
	linter := &support.Linter{
		ChartDir: filepath.Join(dir, ch.Metadata.Name),
	}
	Templates(linter, ch.Values, namespace, strict)
	if len(linter.Messages) != 0 {
		t.Errorf("expected zero messages, got %d", len(linter.Messages))
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %q", i, msg)
		}
	}
}

func TestValidateMatchSelector(t *testing.T) {
	md := &K8sYamlStruct{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: k8sYamlMetadata{
			Name: "mydeployment",
		},
	}
	manifest := `
	apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
	`
	if err := validateMatchSelector(md, manifest); err != nil {
		t.Error(err)
	}
	manifest = `
	apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchExpressions:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
	`
	if err := validateMatchSelector(md, manifest); err != nil {
		t.Error(err)
	}
	manifest = `
	apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
	`
	if err := validateMatchSelector(md, manifest); err == nil {
		t.Error("expected Deployment with no selector to fail")
	}
}

func TestValidateTopIndentLevel(t *testing.T) {
	for doc, shouldFail := range map[string]bool{
		// Should not fail
		"\n\n\n\t\n   \t\n":          false,
		"apiVersion:foo\n  bar:baz":  false,
		"\n\n\napiVersion:foo\n\n\n": false,
		// Should fail
		"  apiVersion:foo":         true,
		"\n\n  apiVersion:foo\n\n": true,
	} {
		if err := validateTopIndentLevel(doc); (err == nil) == shouldFail {
			t.Errorf("Expected %t for %q", shouldFail, doc)
		}
	}

}

// TestEmptyWithCommentsManifests checks the lint is not failing against empty manifests that contains only comments
// See https://github.com/helm/helm/issues/8621
func TestEmptyWithCommentsManifests(t *testing.T) {
	mychart := chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v2",
			Name:       "emptymanifests",
			Version:    "0.1.0",
			Icon:       "satisfy-the-linting-gods.gif",
		},
		Templates: []*chart.File{
			{
				Name: "templates/empty-with-comments.yaml",
				Data: []byte("#@formatter:off\n"),
			},
		},
	}
	tmpdir := ensure.TempDir(t)
	defer os.RemoveAll(tmpdir)

	if err := chartutil.SaveDir(&mychart, tmpdir); err != nil {
		t.Fatal(err)
	}

	linter := support.Linter{ChartDir: filepath.Join(tmpdir, mychart.Name())}
	Templates(&linter, values, namespace, strict)
	if l := len(linter.Messages); l > 0 {
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %s", i, msg)
		}
		t.Fatalf("Expected 0 lint errors, got %d", l)
	}
}
