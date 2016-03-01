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

package manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes/deployment-manager/pkg/common"
	"github.com/kubernetes/deployment-manager/pkg/util"

	"github.com/ghodss/yaml"
)

type mockResolver struct {
	responses [][]*common.ImportFile
	t         *testing.T
}

func (r *mockResolver) ResolveTypes(c *common.Configuration, i []*common.ImportFile) ([]*common.ImportFile, error) {
	if len(r.responses) < 1 {
		return nil, nil
	}

	ret := r.responses[0]
	r.responses = r.responses[1:]
	return ret, nil
}

var validTemplateTestCaseData = common.Template{
	Name:    "TestTemplate",
	Content: string(validContentTestCaseData),
	Imports: validImportFilesTestCaseData,
}

var validContentTestCaseData = []byte(`
imports:
- path: test-type.py
resources:
- name: test
  type: test-type.py
  properties:
    test-property: test-value
`)

var validImportFilesTestCaseData = []*common.ImportFile{
	{
		Name:    "test-type.py",
		Content: "test-type.py validTemplateTestCaseData content",
	},
	{
		Name:    "test.py",
		Content: "test.py validTemplateTestCaseData content",
	},
	{
		Name:    "test2.py",
		Content: "test2.py validTemplateTestCaseData content",
	},
}

var validConfigTestCaseData = []byte(`
resources:
- name: test-service
  properties:
    test-property: test-value
  type: Service
- name: test-rc
  properties:
    test-property: test-value
  type: ReplicationController
- name: test3-service
  properties:
    test-property: test-value
  type: Service
- name: test3-rc
  properties:
    test-property: test-value
  type: ReplicationController
- name: test4-service
  properties:
    test-property: test-value
  type: Service
- name: test4-rc
  properties:
    test-property: test-value
  type: ReplicationController
`)

var validLayoutTestCaseData = []byte(`
resources:
- name: test
  properties:
    test-property: test-value
  resources:
  - name: test-service
    type: Service
  - name: test-rc
    type: ReplicationController
  type: test-type.py
- name: test2
  properties: null
  resources:
  - name: test3
    properties:
      test-property: test-value
    resources:
    - name: test3-service
      type: Service
    - name: test3-rc
      type: ReplicationController
    type: test-type.py
  - name: test4
    properties:
      test-property: test-value
    resources:
    - name: test4-service
      type: Service
    - name: test4-rc
      type: ReplicationController
    type: test-type.py
  type: test2.jinja
`)

var validResponseTestCaseData = ExpansionResponse{
	Config: string(validConfigTestCaseData),
	Layout: string(validLayoutTestCaseData),
}

var roundTripContent = `
config:
  resources:
  - name: test
    type: test.py
    properties:
      test: test
`

var roundTripExpanded = `
resources:
- name: test2
  type: test2.py
  properties:
    test: test
`

var roundTripLayout = `
resources:
- name: test
  type: test.py
  properties:
    test: test
  resources:
  - name: test2
    type: test2.py
    properties:
      test: test
`

var roundTripExpanded2 = `
resources:
- name: test3
  type: Service
  properties:
    test: test
`

var roundTripLayout2 = `
resources:
- name: test2
  type: test2.py
  properties:
    test: test
  resources:
  - name: test3
    type: Service
    properties:
      test: test
`

var finalExpanded = `
config:
  resources:
  - name: test3
    type: Service
    properties:
      test: test
layout:
  resources:
  - name: test
    type: test.py
    properties:
      test: test
    resources:
    - name: test2
      type: test2.py
      properties:
        test: test
      resources:
      - name: test3
        type: Service
        properties:
          test: test
`

var roundTripTemplate = common.Template{
	Name:    "TestTemplate",
	Content: roundTripContent,
	Imports: nil,
}

type ExpanderTestCase struct {
	Description   string
	Error         string
	Handler       func(w http.ResponseWriter, r *http.Request)
	Resolver      TypeResolver
	ValidResponse *ExpandedTemplate
}

func TestExpandTemplate(t *testing.T) {
	roundTripResponse := &ExpandedTemplate{}
	if err := yaml.Unmarshal([]byte(finalExpanded), roundTripResponse); err != nil {
		panic(err)
	}

	tests := []ExpanderTestCase{
		{
			"expect success for ExpandTemplate",
			"",
			expanderSuccessHandler,
			&mockResolver{},
			getValidResponse(t, "expect success for ExpandTemplate"),
		},
		{
			"expect error for ExpandTemplate",
			"cannot expand template",
			expanderErrorHandler,
			&mockResolver{},
			nil,
		},
		{
			"expect success for ExpandTemplate with two expansions",
			"",
			roundTripHandler,
			&mockResolver{[][]*common.ImportFile{
				{},
				{&common.ImportFile{Name: "test.py"}},
			}, t},
			roundTripResponse,
		},
	}

	for _, etc := range tests {
		ts := httptest.NewServer(http.HandlerFunc(etc.Handler))
		defer ts.Close()

		expander := NewExpander(ts.URL, etc.Resolver)
		actualResponse, err := expander.ExpandTemplate(&validTemplateTestCaseData)
		if err != nil {
			message := err.Error()
			if etc.Error == "" {
				t.Errorf("Error in test case %s when there should not be.", etc.Description)
			}
			if !strings.Contains(message, etc.Error) {
				t.Errorf("error in test case:%s:%s\n", etc.Description, message)
			}
		} else {
			if etc.Error != "" {
				t.Errorf("expected error:%s\ndid not occur in test case:%s\n",
					etc.Error, etc.Description)
			}

			expectedResponse := etc.ValidResponse
			if !reflect.DeepEqual(expectedResponse, actualResponse) {
				t.Errorf("error in test case:%s:\nwant:%s\nhave:%s\n",
					etc.Description, util.ToYAMLOrError(expectedResponse), util.ToYAMLOrError(actualResponse))
			}
		}
	}
}

func getValidResponse(t *testing.T, description string) *ExpandedTemplate {
	response, err := validResponseTestCaseData.Unmarshal()
	if err != nil {
		t.Errorf("cannot unmarshal valid response for test case '%s': %s\n", description, err)
	}

	return response
}

func expanderErrorHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	http.Error(w, "something failed", http.StatusInternalServerError)
}

var roundTripResponse = ExpansionResponse{
	Config: roundTripExpanded,
	Layout: roundTripLayout,
}

var roundTripResponse2 = ExpansionResponse{
	Config: roundTripExpanded2,
	Layout: roundTripLayout2,
}

var roundTripResponses = []ExpansionResponse{
	roundTripResponse,
	roundTripResponse2,
}

func roundTripHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	handler := "expandybird: expand"
	util.LogHandlerEntry(handler, r)
	util.LogHandlerExitWithJSON(handler, w, roundTripResponses[0], http.StatusOK)
	roundTripResponses = roundTripResponses[1:]
}

func expanderSuccessHandler(w http.ResponseWriter, r *http.Request) {
	handler := "expandybird: expand"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		status := fmt.Sprintf("cannot read request body:%s", err)
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	template := &common.Template{}
	if err := json.Unmarshal(body, template); err != nil {
		status := fmt.Sprintf("cannot unmarshal request body:%s\n%s\n", err, body)
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	if !reflect.DeepEqual(validTemplateTestCaseData, *template) {
		status := fmt.Sprintf("error in http handler:\nwant:%s\nhave:%s\n",
			util.ToJSONOrError(validTemplateTestCaseData), util.ToJSONOrError(template))
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	util.LogHandlerExitWithJSON(handler, w, validResponseTestCaseData, http.StatusOK)
}
