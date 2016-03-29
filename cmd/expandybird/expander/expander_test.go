/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package expander

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/expansion"
)

var expanderName = "../../../expansion/expansion.py"

// content provides an easy way to provide file content verbatim in tests.
func content(lines []string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

func getChartNameFromPC(pc uintptr) string {
	rf := runtime.FuncForPC(pc)
	fn := rf.Name()
	bn := filepath.Base(fn)
	split := strings.Split(bn, ".")
	if len(split) > 1 {
		split = split[1:]
	}

	cn := fmt.Sprintf("%s-1.2.3.tgz", split[0])
	return cn
}

func getChartURLFromPC(pc uintptr) string {
	cn := getChartNameFromPC(pc)
	cu := fmt.Sprintf("gs://kubernetes-charts-testing/%s", cn)
	return cu
}

func getTestChartName(t *testing.T) string {
	pc, _, _, _ := runtime.Caller(1)
	cu := getChartURLFromPC(pc)
	cl, err := chart.Parse(cu)
	if err != nil {
		t.Fatalf("cannot parse chart reference %s: %s", cu, err)
	}

	return cl.Name
}

func getTestChartURL() string {
	pc, _, _, _ := runtime.Caller(1)
	cu := getChartURLFromPC(pc)
	return cu
}

func testExpansion(t *testing.T, req *expansion.ServiceRequest,
	expResponse *expansion.ServiceResponse, expError string) {
	backend := NewExpander(expanderName)
	response, err := backend.ExpandChart(req)
	if err != nil {
		message := err.Error()
		if expResponse != nil || !strings.Contains(message, expError) {
			t.Fatalf("unexpected error: %s\n", message)
		}
	} else {
		if expResponse == nil {
			t.Fatalf("expected error did not occur: %s\n", expError)
		}
		if !reflect.DeepEqual(response, expResponse) {
			message := fmt.Sprintf(
				"want:\n%s\nhave:\n%s\n", expResponse, response)
			t.Fatalf("output mismatch:\n%s\n", message)
		}
	}
}

var pyExpander = &chart.Expander{
	Name:       "ExpandyBird",
	Entrypoint: "templates/main.py",
}

var jinjaExpander = &chart.Expander{
	Name:       "ExpandyBird",
	Entrypoint: "templates/main.jinja",
}

func TestEmptyJinja(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path:    "templates/main.jinja",
						Content: content([]string{"resources:"}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{},
		},
		"", // Error
	)
}

func TestEmptyPython(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: pyExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.py",
						Content: content([]string{
							"def GenerateConfig(ctx):",
							"  return 'resources:'",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{},
		},
		"", // Error
	)
}

func TestSimpleJinja(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"- name: foo",
							"  type: bar",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"type": "bar",
				},
			},
		},
		"", // Error
	)
}

func TestSimplePython(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: pyExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.py",
						Content: content([]string{
							"def GenerateConfig(ctx):",
							"  return '''resources:",
							"- name: foo",
							"  type: bar",
							"'''",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"type": "bar",
				},
			},
		},
		"", // Error
	)
}

func TestPropertiesJinja(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop2": "foo",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"- name: foo",
							"  type: {{ properties.prop2 }}",
							"  properties:",
							"    something: {{ properties.prop1 }}",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"properties": map[string]interface{}{
						"something": 3.0,
					},
					"type": "foo",
				},
			},
		},
		"", // Error
	)
}

func TestPropertiesPython(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop2": "foo",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: pyExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.py",
						Content: content([]string{
							"def GenerateConfig(ctx):",
							"  return '''resources:",
							"- name: foo",
							"  type: %(prop2)s",
							"  properties:",
							"    something: %(prop1)s",
							"''' % ctx.properties",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"properties": map[string]interface{}{
						"something": 3.0,
					},
					"type": "foo",
				},
			},
		},
		"", // Error
	)
}

func TestMultiFileJinja(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path:    "templates/main.jinja",
						Content: content([]string{"{% include 'templates/secondary.jinja' %}"}),
					},
					{
						Path: "templates/secondary.jinja",
						Content: content([]string{
							"resources:",
							"- name: foo",
							"  type: bar",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"type": "bar",
				},
			},
		},
		"", // Error
	)
}

var schemaContent = content([]string{
	`{`,
	`    "required": ["prop1", "prop2"],`,
	`    "additionalProperties": false,`,
	`    "properties": {`,
	`        "prop1": {`,
	`            "description": "Nice description.",`,
	`            "type": "integer"`,
	`        },`,
	`        "prop2": {`,
	`            "description": "Nice description.",`,
	`            "type": "string"`,
	`        }`,
	`    }`,
	`}`,
})

func TestSchema(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop2": "foo",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
					Schema:   "Schema.yaml",
				},
				Members: []*chart.Member{
					{
						Path:    "Schema.yaml",
						Content: schemaContent,
					},
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"- name: foo",
							"  type: {{ properties.prop2 }}",
							"  properties:",
							"    something: {{ properties.prop1 }}",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"properties": map[string]interface{}{
						"something": 3.0,
					},
					"type": "foo",
				},
			},
		},
		"", // Error
	)
}

func TestSchemaFail(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop3": "foo",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
					Schema:   "Schema.yaml",
				},
				Members: []*chart.Member{
					{
						Path:    "Schema.yaml",
						Content: schemaContent,
					},
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"- name: foo",
							"  type: {{ properties.prop2 }}",
							"  properties:",
							"    something: {{ properties.prop1 }}",
						}),
					},
				},
			},
		},
		nil, // Response.
		"Invalid properties for",
	)
}

func TestMultiFileJinjaMissing(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path:    "templates/main.jinja",
						Content: content([]string{"{% include 'templates/secondary.jinja' %}"}),
					},
				},
			},
		},
		nil, // Response
		"TemplateNotFound: templates/secondary.jinja",
	)
}

func TestMultiFilePython(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: pyExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.py",
						Content: content([]string{
							"from templates import second",
							"import templates.third",
							"def GenerateConfig(ctx):",
							"  t2 = second.Gen()",
							"  t3 = templates.third.Gen()",
							"  return t2",
						}),
					},
					{
						Path: "templates/second.py",
						Content: content([]string{
							"def Gen():",
							"  return '''resources:",
							"- name: foo",
							"  type: bar",
							"'''",
						}),
					},
					{
						Path: "templates/third.py",
						Content: content([]string{
							"def Gen():",
							"  return '''resources:",
							"- name: foo",
							"  type: bar",
							"'''",
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name": "foo",
					"type": "bar",
				},
			},
		},
		"", // Error
	)
}

func TestMultiFilePythonMissing(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: pyExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.py",
						Content: content([]string{
							"from templates import second",
						}),
					},
				},
			},
		},
		nil, // Response
		"cannot import name second", // Error
	)
}

func TestWrongChartName(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     "WrongName",
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path:    "templates/main.jinja",
						Content: content([]string{"resources:"}),
					},
				},
			},
		},
		nil, // Response
		"does not match provided chart",
	)
}

func TestEntrypointNotFound(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{},
			},
		},
		nil, // Response
		"The entrypoint in the chart.yaml cannot be found",
	)
}

func TestMalformedResource(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"fail",
						}),
					},
				},
			},
		},
		nil, // Response
		"could not found expected ':'", // [sic]
	)
}

func TestResourceNoName(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"- type: bar",
						}),
					},
				},
			},
		},
		nil, // Response.
		"Resource does not have a name",
	)
}

func TestResourceNoType(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: getTestChartURL(),
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     getTestChartName(t),
					Expander: jinjaExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.jinja",
						Content: content([]string{
							"resources:",
							"- name: foo",
						}),
					},
				},
			},
		},
		nil, // Response.
		"Resource does not have type defined",
	)
}
