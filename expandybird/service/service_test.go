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

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/kubernetes/deployment-manager/expandybird/expander"
	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/util"

	restful "github.com/emicklei/go-restful"
)

func GetTemplateReader(t *testing.T, description string, templateFileName string) io.Reader {
	template, err := expander.NewTemplateFromFileNames(templateFileName, importFileNames)
	if err != nil {
		t.Errorf("cannot create template for test case (%s): %s\n", err, description)
	}

	templateData, err := json.Marshal(template)
	if err != nil {
		t.Errorf("cannot marshal template for test case (%s): %s\n", err, description)
	}

	reader := bytes.NewReader(templateData)
	return reader
}

func GetOutputString(t *testing.T, description string) string {
	output, err := ioutil.ReadFile(outputFileName)
	if err != nil {
		t.Errorf("cannot read output file for test case (%s): %s\n", err, description)
	}

	return string(output)
}

const (
	httpGETMethod      = "GET"
	httpPOSTMethod     = "POST"
	validServiceURL    = "/expand"
	invalidServiceURL  = "http://localhost:8080/invalidurlpath"
	jsonContentType    = "application/json"
	invalidContentType = "invalid/content-type"
	inputFileName      = "../test/ValidContent.yaml"
	outputFileName     = "../test/ExpectedOutput.yaml"
)

var importFileNames = []string{
	"../test/replicatedservice.py",
}

type ServiceWrapperTestCase struct {
	Description    string
	HTTPMethod     string
	ServiceURLPath string
	ContentType    string
	StatusCode     int
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
	backend := expander.NewExpander("../expansion/expansion.py")
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
	Description      string
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
