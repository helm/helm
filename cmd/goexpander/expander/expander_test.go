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
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/expansion"
)

// content provides an easy way to provide file content verbatim in tests.
func content(lines []string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

func testExpansion(t *testing.T, req *expansion.ServiceRequest,
	expResponse *expansion.ServiceResponse, expError string) {
	backend := NewExpander()
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

var goExpander = &chart.Expander{
	Name:       "GoTemplating",
	Entrypoint: "templates/main.py",
}

func TestEmpty(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: "gs://kubernetes-charts-testing/Test-1.2.3.tgz",
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     "Test",
					Expander: goExpander,
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{},
		},
		"", // Error
	)
}

func TestSingle(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: "gs://kubernetes-charts-testing/Test-1.2.3.tgz",
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     "Test",
					Expander: goExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.yaml",
						Content: content([]string{
							"name: foo",
							"type: bar",
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

func TestProperties(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: "gs://kubernetes-charts-testing/Test-1.2.3.tgz",
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop2": "foo",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     "Test",
					Expander: goExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/main.yaml",
						Content: content([]string{
							"name: foo",
							"type: {{ .prop2 }}",
							"properties:",
							"  something: {{ .prop1 }}",
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

func TestComplex(t *testing.T) {
	testExpansion(
		t,
		&expansion.ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "test_invocation",
				Type: "gs://kubernetes-charts-testing/Test-1.2.3.tgz",
				Properties: map[string]interface{}{
					"DatabaseName": "mydb",
					"NumRepicas":   3,
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Name:     "Test",
					Expander: goExpander,
				},
				Members: []*chart.Member{
					{
						Path: "templates/bar.tmpl",
						Content: content([]string{
							`{{ template "banana" . }}`,
						}),
					},
					{
						Path: "templates/base.tmpl",
						Content: content([]string{
							`{{ define "apple" }}`,
							`name: Abby`,
							`kind: Apple`,
							`dbname: {{default "whatdb" .DatabaseName}}`,
							`{{ end }}`,
							``,
							`{{ define "banana" }}`,
							`name: Bobby`,
							`kind: Banana`,
							`dbname: {{default "whatdb" .DatabaseName}}`,
							`{{ end }}`,
						}),
					},
					{
						Path: "templates/foo.tmpl",
						Content: content([]string{
							`---`,
							`foo:`,
							`  bar: baz`,
							`---`,
							`{{ template "apple" . }}`,
							`---`,
							`{{ template "apple" . }}`,
							`...`,
						}),
					},
					{
						Path: "templates/docs.txt",
						Content: content([]string{
							`{{/*`,
							`File contains only a comment.`,
							`Suitable for documentation within templates/`,
							`*/}}`,
						}),
					},
					{
						Path: "templates/docs2.txt",
						Content: content([]string{
							`# File contains only a comment.`,
							`# Suitable for documentation within templates/`,
						}),
					},
				},
			},
		},
		&expansion.ServiceResponse{
			Resources: []interface{}{
				map[string]interface{}{
					"name":   "Bobby",
					"kind":   "Banana",
					"dbname": "mydb",
				},
				map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
				},
				map[string]interface{}{
					"name":   "Abby",
					"kind":   "Apple",
					"dbname": "mydb",
				},
				map[string]interface{}{
					"name":   "Abby",
					"kind":   "Apple",
					"dbname": "mydb",
				},
			},
		},
		"", // Error
	)
}
