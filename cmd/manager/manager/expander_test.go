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

	"github.com/ghodss/yaml"
	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/expansion"
	"github.com/kubernetes/helm/pkg/repo"
	"github.com/kubernetes/helm/pkg/util"
)

var (
	TestRepoBucket   = "kubernetes-charts-testing"
	TestRepoURL      = "gs://" + TestRepoBucket
	TestChartName    = "frobnitz"
	TestChartVersion = "0.0.1"
	TestArchiveName  = TestChartName + "-" + TestChartVersion + ".tgz"
	TestResourceType = TestRepoURL + "/" + TestArchiveName
)

var validResponseTestCaseData = []byte(`
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
- name: test_invocation
  resources:
  - name: test-service
    type: Service
  - name: test-rc
    type: ReplicationController
  - name: test3-service
    type: Service
  - name: test3-rc
    type: ReplicationController
  - name: test4-service
    type: Service
  - name: test4-rc
    type: ReplicationController
  type: gs://kubernetes-charts-testing/frobnitz-0.0.1.tgz
`)

/*
[]byte(`
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

var roundTripContent = `
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

var roundTripResponse = &ExpandedConfiguration{
	Config: roundTripExpanded,
}

var roundTripResponse2 = &ExpandedConfiguration{
	Config: roundTripExpanded2,
}

var roundTripResponses = []*ExpandedConfiguration{
	roundTripResponse,
	roundTripResponse2,
}
*/

type mockRepoProvider struct {
}

func (m *mockRepoProvider) GetChartByReference(reference string) (*chart.Chart, repo.IChartRepo, error) {
	return &chart.Chart{}, nil, nil
}

func (m *mockRepoProvider) GetRepoByChartURL(URL string) (repo.IChartRepo, error) {
	return nil, nil
}

func (m *mockRepoProvider) GetRepoByURL(URL string) (repo.IChartRepo, error) {
	return nil, nil
}

type ExpanderTestCase struct {
	Description   string
	Error         string
	Handler       func(w http.ResponseWriter, r *http.Request)
	ValidResponse *ExpandedConfiguration
}

func TestExpandTemplate(t *testing.T) {
	//	roundTripResponse := &ExpandedConfiguration{}
	//	if err := yaml.Unmarshal([]byte(finalExpanded), roundTripResponse); err != nil {
	//		panic(err)
	//	}

	tests := []ExpanderTestCase{
		{
			"expect success for ExpandConfiguration",
			"",
			expanderSuccessHandler,
			getValidExpandedConfiguration(),
		},
		{
			"expect error for ExpandConfiguration",
			"simulated failure",
			expanderErrorHandler,
			nil,
		},
	}

	for _, etc := range tests {
		ts := httptest.NewServer(http.HandlerFunc(etc.Handler))
		defer ts.Close()

		expander := NewExpander(ts.URL, nil)
		resource := &common.Resource{
			Name: "test_invocation",
			Type: TestResourceType,
		}

		conf := &common.Configuration{
			Resources: []*common.Resource{
				resource,
			},
		}

		actualResponse, err := expander.ExpandConfiguration(conf)
		if err != nil {
			message := err.Error()
			if etc.Error == "" {
				t.Errorf("unexpected error in test case %s: %s", etc.Description, err)
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

func getValidServiceResponse() *common.Configuration {
	conf := &common.Configuration{}
	if err := yaml.Unmarshal(validResponseTestCaseData, conf); err != nil {
		panic(fmt.Errorf("cannot unmarshal valid response: %s\n", err))
	}

	return conf
}

func getValidExpandedConfiguration() *ExpandedConfiguration {
	conf := getValidServiceResponse()
	layout := &common.Layout{}
	if err := yaml.Unmarshal(validLayoutTestCaseData, layout); err != nil {
		panic(fmt.Errorf("cannot unmarshal valid response: %s\n", err))
	}

	return &ExpandedConfiguration{Config: conf, Layout: layout}

}

func expanderErrorHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	http.Error(w, "simulated failure", http.StatusInternalServerError)
}

/*
func roundTripHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	handler := "expandybird: expand"
	util.LogHandlerEntry(handler, r)
	if len(roundTripResponses) < 1 {
		http.Error(w, "Too many calls to round trip handler", http.StatusInternalServerError)
		return
	}

	util.LogHandlerExitWithJSON(handler, w, roundTripResponses[0], http.StatusOK)
	roundTripResponses = roundTripResponses[1:]
}
*/

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

	svcReq := &expansion.ServiceRequest{}
	if err := json.Unmarshal(body, svcReq); err != nil {
		status := fmt.Sprintf("cannot unmarshal request body:%s\n%s\n", err, body)
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	/*
		if !reflect.DeepEqual(validRequestTestCaseData, *svcReq) {
			status := fmt.Sprintf("error in http handler:\nwant:%s\nhave:%s\n",
				util.ToJSONOrError(validRequestTestCaseData), util.ToJSONOrError(template))
			http.Error(w, status, http.StatusInternalServerError)
			return
		}
	*/

	svcResp := getValidServiceResponse()
	util.LogHandlerExitWithJSON(handler, w, svcResp, http.StatusOK)
}
