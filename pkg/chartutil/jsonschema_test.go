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

package chartutil

import (
	"io/ioutil"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
)

func TestValidateAgainstSingleSchema(t *testing.T) {
	values, err := ReadValuesFile("./testdata/test-values.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schema, err := ioutil.ReadFile("./testdata/test-values.schema.json")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}

	if err := ValidateAgainstSingleSchema(values, schema); err != nil {
		t.Errorf("Error validating Values against Schema: %s", err)
	}
}

func TestValidateAgainstSingleSchemaNegative(t *testing.T) {
	values, err := ReadValuesFile("./testdata/test-values-negative.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schema, err := ioutil.ReadFile("./testdata/test-values.schema.json")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}

	var errString string
	if err := ValidateAgainstSingleSchema(values, schema); err == nil {
		t.Fatalf("Expected an error, but got nil")
	} else {
		errString = err.Error()
	}

	expectedErrString := `- (root): employmentInfo is required
- age: Must be greater than or equal to 0
`
	if errString != expectedErrString {
		t.Errorf("Error string :\n`%s`\ndoes not match expected\n`%s`", errString, expectedErrString)
	}
}

const subchartSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Values",
  "type": "object",
  "properties": {
    "age": {
      "description": "Age",
      "minimum": 0,
      "type": "integer"
    }
  },
  "required": [
    "age"
  ]
}
`

func TestValidateAgainstSchema(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Schema: subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "chrt",
		},
	}
	chrt.AddDependency(subchart)

	vals := map[string]interface{}{
		"name": "John",
		"subchart": map[string]interface{}{
			"age": 25,
		},
	}

	if err := ValidateAgainstSchema(chrt, vals); err != nil {
		t.Errorf("Error validating Values against Schema: %s", err)
	}
}

func TestValidateAgainstSchemaNegative(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Schema: subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "chrt",
		},
	}
	chrt.AddDependency(subchart)

	vals := map[string]interface{}{
		"name":     "John",
		"subchart": map[string]interface{}{},
	}

	var errString string
	if err := ValidateAgainstSchema(chrt, vals); err == nil {
		t.Fatalf("Expected an error, but got nil")
	} else {
		errString = err.Error()
	}

	expectedErrString := `subchart:
- (root): age is required
`
	if errString != expectedErrString {
		t.Errorf("Error string :\n`%s`\ndoes not match expected\n`%s`", errString, expectedErrString)
	}
}
