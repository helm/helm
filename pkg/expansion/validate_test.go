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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
)

func testPropertiesValidation(t *testing.T, req *ServiceRequest, expRequest *ServiceRequest, expError string) {
	modifiedRequest, err := ValidateProperties(req)
	if err != nil {
		message := err.Error()
		if expRequest != nil || !strings.Contains(message, expError) {
			t.Fatalf("unexpected error: %v\n", err)
		}
	} else {
		if expRequest == nil {
			t.Fatalf("expected error did not occur: %s\n", expError)
		}
		if !reflect.DeepEqual(modifiedRequest, expRequest) {
			message := fmt.Sprintf("want:\n%s\nhave:\n%s\n", expRequest, modifiedRequest)
			t.Fatalf("output mismatch:\n%s\n", message)
		}
	}
}

func TestNoSchema(t *testing.T) {
	req := &ServiceRequest{
		ChartInvocation: &common.Resource{
			Properties: map[string]interface{}{
				"prop1": 3.0,
				"prop2": "foo",
			},
		},
		Chart: &chart.Content{
			Chartfile: &chart.Chartfile{},
			Members:   []*chart.Member{},
		},
	}
	testPropertiesValidation(t, req, req, "") // Returns it unchanged.
}

func TestSchemaNotFound(t *testing.T) {
	testPropertiesValidation(
		t,
		&ServiceRequest{
			ChartInvocation: &common.Resource{
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop2": "foo",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Schema: "Schema.yaml",
				},
			},
		},
		nil, // No response to check.
		"The schema referenced from the Chart.yaml cannot be found: Schema.yaml",
	)
}

var schemaContent = []byte(`
  required: ["prop2"]
  additionalProperties: false
  properties:
    prop1:
      description: Nice description.
      type: integer
      default: 42
    prop2:
      description: Nice description.
      type: string
`)

func TestSchema(t *testing.T) {
	req := &ServiceRequest{
		ChartInvocation: &common.Resource{
			Properties: map[string]interface{}{
				"prop1": 3.0,
				"prop2": "foo",
			},
		},
		Chart: &chart.Content{
			Chartfile: &chart.Chartfile{
				Schema: "Schema.yaml",
			},
			Members: []*chart.Member{
				{
					Path:    "Schema.yaml",
					Content: schemaContent,
				},
			},
		},
	}
	// No defaults, returns it unchanged:
	testPropertiesValidation(t, req, req, "")
}

func TestBadProperties(t *testing.T) {
	testPropertiesValidation(
		t,
		&ServiceRequest{
			ChartInvocation: &common.Resource{
				Properties: map[string]interface{}{
					"prop1": 3.0,
					"prop3": map[string]interface{}{},
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Schema: "Schema.yaml",
				},
				Members: []*chart.Member{
					{
						Path:    "Schema.yaml",
						Content: schemaContent,
					},
				},
			},
		},
		nil,
		"Properties failed validation:",
	)
}

func TestDefault(t *testing.T) {
	testPropertiesValidation(
		t,
		&ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "TestName",
				Type: "TestType",
				Properties: map[string]interface{}{
					"prop2": "ok",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Schema: "Schema.yaml",
				},
				Members: []*chart.Member{
					{
						Path:    "Schema.yaml",
						Content: schemaContent,
					},
				},
			},
		},
		&ServiceRequest{
			ChartInvocation: &common.Resource{
				Name: "TestName",
				Type: "TestType",
				Properties: map[string]interface{}{
					"prop1": 42.0,
					"prop2": "ok",
				},
			},
			Chart: &chart.Content{
				Chartfile: &chart.Chartfile{
					Schema: "Schema.yaml",
				},
				Members: []*chart.Member{
					{
						Path:    "Schema.yaml",
						Content: schemaContent,
					},
				},
			},
		},
		"", // Error
	)
}
