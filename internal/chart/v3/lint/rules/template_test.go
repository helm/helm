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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	"helm.sh/helm/v4/pkg/chart/common"
)

const templateTestBasedir = "./testdata/albatross"

func TestValidateAllowedExtension(t *testing.T) {
	var failTest = []string{"/foo", "/test.toml"}
	for _, test := range failTest {
		require.ErrorContains(t, validateAllowedExtension(test), "Valid extensions are .yaml, .yml, .tpl, or .txt", "validateAllowedExtension('%s') to return \"Valid extensions are .yaml, .yml, .tpl, or .txt\", got no error", test)
	}
	var successTest = []string{"/foo.yaml", "foo.yaml", "foo.tpl", "/foo/bar/baz.yaml", "NOTES.txt"}
	for _, test := range successTest {
		assert.NoError(t, validateAllowedExtension(test), "validateAllowedExtension('%s') to return no error", test)
	}
}

var values = map[string]any{"nameOverride": "", "httpPort": 80}

const namespace = "testNamespace"
const strict = false

func TestTemplateParsing(t *testing.T) {
	linter := support.Linter{ChartDir: templateTestBasedir}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	require.Len(t, res, 1, "Expected one error, got %d, %v", len(res), res)
	assert.ErrorContains(t, res[0].Err, "deliberateSyntaxError")
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

	require.Empty(t, res, "Expected no error, got %d, %v", len(res), res)
}

func TestMultiTemplateFail(t *testing.T) {
	linter := support.Linter{ChartDir: "./testdata/multi-template-fail"}
	Templates(&linter, values, namespace, strict)
	res := linter.Messages

	require.Len(t, res, 1, "Expected 1 error, got %d, %v", len(res), res)

	assert.ErrorContains(t, res[0].Err, "object name does not conform to Kubernetes naming requirements")
}

func TestValidateMetadataName(t *testing.T) {
	tests := []struct {
		obj     *k8sYamlStruct
		wantErr bool
	}{
		// Most kinds use IsDNS1123Subdomain.
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: ""}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "foo.bar1234baz.seventyone"}}, false},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "FOO"}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "123baz"}}, false},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "foo.BAR.baz"}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "one-two"}}, false},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "-two"}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "one_two"}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "a..b"}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "%^&#$%*@^*@&#^"}}, true},
		{&k8sYamlStruct{Kind: "Pod", Metadata: k8sYamlMetadata{Name: "operator:pod"}}, true},
		{&k8sYamlStruct{Kind: "ServiceAccount", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "ServiceAccount", Metadata: k8sYamlMetadata{Name: "foo.bar1234baz.seventyone"}}, false},
		{&k8sYamlStruct{Kind: "ServiceAccount", Metadata: k8sYamlMetadata{Name: "FOO"}}, true},
		{&k8sYamlStruct{Kind: "ServiceAccount", Metadata: k8sYamlMetadata{Name: "operator:sa"}}, true},

		// Service uses IsDNS1035Label.
		{&k8sYamlStruct{Kind: "Service", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "Service", Metadata: k8sYamlMetadata{Name: "123baz"}}, true},
		{&k8sYamlStruct{Kind: "Service", Metadata: k8sYamlMetadata{Name: "foo.bar"}}, true},

		// Namespace uses IsDNS1123Label.
		{&k8sYamlStruct{Kind: "Namespace", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "Namespace", Metadata: k8sYamlMetadata{Name: "123baz"}}, false},
		{&k8sYamlStruct{Kind: "Namespace", Metadata: k8sYamlMetadata{Name: "foo.bar"}}, true},
		{&k8sYamlStruct{Kind: "Namespace", Metadata: k8sYamlMetadata{Name: "foo-bar"}}, false},

		// CertificateSigningRequest has no validation.
		{&k8sYamlStruct{Kind: "CertificateSigningRequest", Metadata: k8sYamlMetadata{Name: ""}}, false},
		{&k8sYamlStruct{Kind: "CertificateSigningRequest", Metadata: k8sYamlMetadata{Name: "123baz"}}, false},
		{&k8sYamlStruct{Kind: "CertificateSigningRequest", Metadata: k8sYamlMetadata{Name: "%^&#$%*@^*@&#^"}}, false},

		// RBAC uses path validation.
		{&k8sYamlStruct{Kind: "Role", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "Role", Metadata: k8sYamlMetadata{Name: "123baz"}}, false},
		{&k8sYamlStruct{Kind: "Role", Metadata: k8sYamlMetadata{Name: "foo.bar"}}, false},
		{&k8sYamlStruct{Kind: "Role", Metadata: k8sYamlMetadata{Name: "operator:role"}}, false},
		{&k8sYamlStruct{Kind: "Role", Metadata: k8sYamlMetadata{Name: "operator/role"}}, true},
		{&k8sYamlStruct{Kind: "Role", Metadata: k8sYamlMetadata{Name: "operator%role"}}, true},
		{&k8sYamlStruct{Kind: "ClusterRole", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "ClusterRole", Metadata: k8sYamlMetadata{Name: "123baz"}}, false},
		{&k8sYamlStruct{Kind: "ClusterRole", Metadata: k8sYamlMetadata{Name: "foo.bar"}}, false},
		{&k8sYamlStruct{Kind: "ClusterRole", Metadata: k8sYamlMetadata{Name: "operator:role"}}, false},
		{&k8sYamlStruct{Kind: "ClusterRole", Metadata: k8sYamlMetadata{Name: "operator/role"}}, true},
		{&k8sYamlStruct{Kind: "ClusterRole", Metadata: k8sYamlMetadata{Name: "operator%role"}}, true},
		{&k8sYamlStruct{Kind: "RoleBinding", Metadata: k8sYamlMetadata{Name: "operator:role"}}, false},
		{&k8sYamlStruct{Kind: "ClusterRoleBinding", Metadata: k8sYamlMetadata{Name: "operator:role"}}, false},

		// Unknown Kind
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: ""}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "foo.bar1234baz.seventyone"}}, false},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "FOO"}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "123baz"}}, false},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "foo.BAR.baz"}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "one-two"}}, false},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "-two"}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "one_two"}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "a..b"}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "%^&#$%*@^*@&#^"}}, true},
		{&k8sYamlStruct{Kind: "FutureKind", Metadata: k8sYamlMetadata{Name: "operator:pod"}}, true},

		// No kind
		{&k8sYamlStruct{Metadata: k8sYamlMetadata{Name: "foo"}}, false},
		{&k8sYamlStruct{Metadata: k8sYamlMetadata{Name: "operator:pod"}}, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.obj.Kind, tt.obj.Metadata.Name), func(t *testing.T) {
			err := validateMetadataName(tt.obj)
			if tt.wantErr {
				require.Error(t, err, "validateMetadataName()")
			} else {
				require.NoError(t, err, "validateMetadataName()")
			}
		})
	}
}

func TestDeprecatedAPIFails(t *testing.T) {
	modTime := time.Now()
	mychart := chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v2",
			Name:       "failapi",
			Version:    "0.1.0",
			Icon:       "satisfy-the-linting-gods.gif",
		},
		Templates: []*common.File{
			{
				Name:    "templates/baddeployment.yaml",
				ModTime: modTime,
				Data:    []byte("apiVersion: apps/v1beta1\nkind: Deployment\nmetadata:\n  name: baddep\nspec: {selector: {matchLabels: {foo: bar}}}"),
			},
			{
				Name:    "templates/goodsecret.yaml",
				ModTime: modTime,
				Data:    []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: goodsecret"),
			},
		},
	}
	tmpdir := t.TempDir()

	require.NoError(t, chartutil.SaveDir(&mychart, tmpdir))

	linter := support.Linter{ChartDir: filepath.Join(tmpdir, mychart.Name())}
	Templates(&linter, values, namespace, strict)
	if !assert.Len(t, linter.Messages, 1) {
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %s", i, msg)
		}
	}
	require.Len(t, linter.Messages, 1, "Expected 1 lint error")

	var err deprecatedAPIError
	require.ErrorAs(t, linter.Messages[0].Err, &err, "Expected error to be of type deprecatedAPIError")
	assert.Equal(t, "apps/v1beta1 Deployment", err.Deprecated, "Surprised to learn that %q is deprecated", err.Deprecated)
}

const manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myval1: {{default "val" .Values.mymap.key1 }}
  myval2: {{default "val" .Values.mymap.key2 }}
`

// TestStrictTemplateParsingMapError is a regression test.
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
		Values: map[string]any{
			"mymap": map[string]string{
				"key1": "val1",
			},
		},
		Templates: []*common.File{
			{
				Name:    "templates/configmap.yaml",
				ModTime: time.Now(),
				Data:    []byte(manifest),
			},
		},
	}
	dir := t.TempDir()
	require.NoError(t, chartutil.SaveDir(&ch, dir))
	linter := &support.Linter{
		ChartDir: filepath.Join(dir, ch.Metadata.Name),
	}
	Templates(linter, ch.Values, namespace, strict)
	if !assert.Empty(t, linter.Messages, "expected zero messages") {
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %q", i, msg)
		}
	}
}

func TestValidateMatchSelector(t *testing.T) {
	md := &k8sYamlStruct{
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
	require.NoError(t, validateMatchSelector(md, manifest))
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
	require.NoError(t, validateMatchSelector(md, manifest))
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
	assert.Error(t, validateMatchSelector(md, manifest), "expected Deployment with no selector to fail")
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
		err := validateTopIndentLevel(doc)
		if shouldFail {
			assert.Errorf(t, err, "Expected %t for %q", shouldFail, doc)
		} else {
			assert.NoErrorf(t, err, "Expected %t for %q", shouldFail, doc)
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
		Templates: []*common.File{
			{
				Name:    "templates/empty-with-comments.yaml",
				ModTime: time.Now(),
				Data:    []byte("#@formatter:off\n"),
			},
		},
	}
	tmpdir := t.TempDir()

	require.NoError(t, chartutil.SaveDir(&mychart, tmpdir))

	linter := support.Linter{ChartDir: filepath.Join(tmpdir, mychart.Name())}
	Templates(&linter, values, namespace, strict)
	if !assert.Empty(t, linter.Messages) {
		for i, msg := range linter.Messages {
			t.Logf("Message %d: %s", i, msg)
		}
	}
	require.Empty(t, linter.Messages, "Expected 0 lint errors")
}
func TestValidateListAnnotations(t *testing.T) {
	md := &k8sYamlStruct{
		APIVersion: "v1",
		Kind:       "List",
		Metadata: k8sYamlMetadata{
			Name: "list",
		},
	}
	manifest := `
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      annotations:
        helm.sh/resource-policy: keep
`

	require.Error(t, validateListAnnotations(md, manifest), "expected list with nested keep annotations to fail")

	manifest = `
apiVersion: v1
kind: List
metadata:
  annotations:
    helm.sh/resource-policy: keep
items:
  - apiVersion: v1
    kind: ConfigMap
`

	require.NoError(t, validateListAnnotations(md, manifest), "List objects keep annotations should pass")
}

func TestIsYamlFileExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.yaml", true},
		{"test.yml", true},
		{"test.txt", false},
		{"test", false},
	}

	for _, test := range tests {
		result := isYamlFileExtension(test.filename)
		assert.Equal(t, test.expected, result, "isYamlFileExtension(%s) = %v; want %v", test.filename, result, test.expected)
	}
}
