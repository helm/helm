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

package expansion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"
)

var (
	testRequest = &ServiceRequest{
		ChartInvocation: &common.Resource{
			Name: "test_invocation",
			Type: "Test Chart",
		},
		Chart: &chart.Content{
			Chartfile: &chart.Chartfile{
				Name: "TestChart",
				Expander: &chart.Expander{
					Name:       "FakeExpander",
					Entrypoint: "None",
				},
			},
			Members: []*chart.Member{
				{
					Path:    "templates/testfile",
					Content: []byte("test"),
				},
			},
		},
	}
	testResponse = &ServiceResponse{
		Resources: []interface{}{"test"},
	}
)

// A FakeExpander returns testResponse if it was given testRequest, otherwise raises an error.
type FakeExpander struct {
}

func (fake *FakeExpander) ExpandChart(req *ServiceRequest) (*ServiceResponse, error) {
	if reflect.DeepEqual(req, testRequest) {
		return testResponse, nil
	}
	return nil, fmt.Errorf("Test Error Response")
}

func wrapReader(value interface{}) (io.Reader, error) {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(valueJSON), nil
}

func GeneralTest(t *testing.T, httpMeth string, url string, contentType string, req *ServiceRequest,
	expResponse *ServiceResponse, expStatus int) {
	service := NewService("127.0.0.1", 8080, &FakeExpander{})
	handlerTester := util.NewHandlerTester(service.container)
	reader, err := wrapReader(testRequest)
	if err != nil {
		t.Fatalf("unexpected error: %s\n", err)
	}
	w, err := handlerTester(httpMeth, url, contentType, reader)
	if err != nil {
		t.Fatalf("unexpected error: %s\n", err)
	}
	var data = w.Body.Bytes()
	if w.Code != expStatus {
		t.Fatalf("wrong status code:\nwant: %d\ngot:  %d\ncontent: %s\n", expStatus, w.Code, data)
	}
	if expResponse != nil {
		var response ServiceResponse
		err = json.Unmarshal(data, &response)
		if err != nil {
			t.Fatalf("Response could not be unmarshalled: %s\nresponse: %s", err, string(data))
		}
		if !reflect.DeepEqual(response, *expResponse) {
			t.Fatalf("Response did not match.\nwant: %s\ngot:  %s\n", expResponse, response)
		}
	}
}

func TestInvalidMethod(t *testing.T) {
	GeneralTest(t, "GET", "/expand", "application/json", nil, nil, http.StatusMethodNotAllowed)
}

func TestInvalidURL(t *testing.T) {
	GeneralTest(t, "POST", "/erroneus", "application/json", testRequest, nil, http.StatusNotFound)
}

func TestInvalidMimeType(t *testing.T) {
	GeneralTest(t, "POST", "/expand", "erroneus", nil, nil, http.StatusUnsupportedMediaType)
}

func TestExpandOK(t *testing.T) {
	GeneralTest(t, "POST", "/expand", "application/json", testRequest, testResponse, http.StatusOK)
}

/*
type ServiceWrapperTestCase struct {
	Description	string
	HTTPMethod	 string
	ServiceURLPath string
	ContentType	string
	StatusCode	 int
}

var ServiceWrapperTestCases = []ServiceWrapperTestCase{
	{
		"expect error for invalid HTTP verb",
		httpGETMethod,
		validServiceURL,
		jsonContentType,
		http.StatusMethodNotAllowed,
	},
	{
		"expect error for invalid URL path",
		httpPOSTMethod,
		invalidServiceURL,
		jsonContentType,
		http.StatusNotFound,
	},
	{
		"expect error for invalid content type",
		httpPOSTMethod,
		validServiceURL,
		invalidContentType,
		http.StatusUnsupportedMediaType,
	},
	{
		"expect success",
		httpPOSTMethod,
		validServiceURL,
		jsonContentType,
		http.StatusOK,
	},
}

func TestServiceWrapper(t *testing.T) {
	backend := expander.NewExpander("../../../expansion/expansion.py")
	wrapper := NewService(NewExpansionHandler(backend))
	container := restful.NewContainer()
	container.ServeMux = http.NewServeMux()
	wrapper.Register(container)
	handlerTester := util.NewHandlerTester(container)
	for _, swtc := range ServiceWrapperTestCases {
		reader := GetTemplateReader(t, swtc.Description, inputFileName)
		w, err := handlerTester(swtc.HTTPMethod, swtc.ServiceURLPath, swtc.ContentType, reader)
		if err != nil {
			t.Errorf("error in test case '%s': %s\n", swtc.Description, err)
		}

		if w.Code != http.StatusOK {
			if w.Code != swtc.StatusCode {
				message := fmt.Sprintf("test returned code:%d, status: %s", w.Code, w.Body.String())
				t.Errorf("error in test case '%s': %s\n", swtc.Description, message)
			}
		} else {
			if swtc.StatusCode != http.StatusOK {
				t.Errorf("expected error did not occur in test case '%s': want: %d have: %d\n",
					swtc.Description, swtc.StatusCode, w.Code)
			}

			body := w.Body.Bytes()
			actualResponse := &expander.ExpansionResponse{}
			if err := json.Unmarshal(body, actualResponse); err != nil {
				t.Errorf("error in test case '%s': %s\n", swtc.Description, err)
			}

			actualResult, err := actualResponse.Unmarshal()
			if err != nil {
				t.Errorf("error in test case '%s': %s\n", swtc.Description, err)
			}

			expectedOutput := GetOutputString(t, swtc.Description)
			expectedResult := expandOutputOrDie(t, expectedOutput, swtc.Description)

			if !reflect.DeepEqual(expectedResult, actualResult) {
				message := fmt.Sprintf("want: %s\nhave: %s\n",
					util.ToYAMLOrError(expectedResult), util.ToYAMLOrError(actualResult))
				t.Errorf("error in test case '%s':\n%s\n", swtc.Description, message)
			}
		}
	}
}

type ExpansionHandlerTestCase struct {
	Description	  string
	TemplateFileName string
}

var ExpansionHandlerTestCases = []ExpansionHandlerTestCase{
	{
		"expect error while expanding template",
		"../test/InvalidFileName.yaml",
	},
	{
		"expect error while marshaling output",
		"../test/InvalidTypeName.yaml",
	},
}

var malformedExpansionOutput = []byte(`
this: is: invalid: yaml:
`)

type mockExpander struct {
}

// ExpandTemplate passes the given configuration to the expander and returns the
// expanded configuration as a string on success.
func (e *mockExpander) ExpandTemplate(template *common.Template) (string, error) {
	switch template.Name {
	case "InvalidFileName.yaml":
		return "", fmt.Errorf("expansion error")
	case "InvalidTypeName.yaml":
		return string(malformedExpansionOutput), nil
	}

	panic("unknown test case")
}

func TestExpansionHandler(t *testing.T) {
	backend := &mockExpander{}
	wrapper := NewService(NewExpansionHandler(backend))
	container := restful.DefaultContainer
	wrapper.Register(container)
	handlerTester := util.NewHandlerTester(container)
	for _, ehtc := range ExpansionHandlerTestCases {
		reader := GetTemplateReader(t, ehtc.Description, ehtc.TemplateFileName)
		w, err := handlerTester(httpPOSTMethod, validServiceURL, jsonContentType, reader)
		if err != nil {
			t.Errorf("error in test case '%s': %s\n", ehtc.Description, err)
		}

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected error did not occur in test case '%s': want: %d have: %d\n",
				ehtc.Description, http.StatusBadRequest, w.Code)
		}
	}
}

func expandOutputOrDie(t *testing.T, output, description string) *expander.ExpansionResult {
	result, err := expander.NewExpansionResult(output)
	if err != nil {
		t.Errorf("cannot expand output for test case '%s': %s\n", description, err)
	}

	return result
}
*/
