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
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/juju/gojsonschema"
	"github.com/kubernetes/helm/pkg/chart"
)

// ValidateRequest does basic sanity checks on the request.
func ValidateRequest(request *ServiceRequest) error {
	if request.ChartInvocation == nil {
		return fmt.Errorf("Request does not have invocation field")
	}
	if request.Chart == nil {
		return fmt.Errorf("Request does not have chart field")
	}

	chartInv := request.ChartInvocation
	chartFile := request.Chart.Chartfile

	l, err := chart.Parse(chartInv.Type)
	if err != nil {
		return fmt.Errorf("cannot parse chart reference %s: %s", chartInv.Type, err)
	}

	if l.Name != chartFile.Name {
		return fmt.Errorf("Chart invocation type (%s) does not match provided chart (%s)", chartInv.Type, chartFile.Name)
	}

	if chartFile.Expander == nil {
		message := fmt.Sprintf("Chart JSON does not have expander field")
		return fmt.Errorf("%s: %s", chartInv.Name, message)
	}

	return nil
}

// ValidateProperties validates the properties in the chart invocation against the schema file in
// the chart itself, which is assumed to be JSONschema.  It also modifies a copy of the request to
// add defaults values if properties are not provided (according to the default field in
// JSONschema), and returns this copy.
func ValidateProperties(request *ServiceRequest) (*ServiceRequest, error) {

	schemaFilename := request.Chart.Chartfile.Schema

	if schemaFilename == "" {
		// No schema, so perform no validation.
		return request, nil
	}

	chartInv := request.ChartInvocation

	var schemaBytes *[]byte
	for _, f := range request.Chart.Members {
		if f.Path == schemaFilename {
			schemaBytes = &f.Content
		}
	}
	if schemaBytes == nil {
		return nil, fmt.Errorf("%s: The schema referenced from the Chart.yaml cannot be found: %s",
			chartInv.Name, schemaFilename)
	}
	var schemaDoc interface{}

	if err := yaml.Unmarshal(*schemaBytes, &schemaDoc); err != nil {
		return nil, fmt.Errorf("%s: %s was not valid YAML: %v",
			chartInv.Name, schemaFilename, err)
	}

	// Build a schema object
	schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(schemaDoc))
	if err != nil {
		return nil, err
	}

	// Do validation
	result, err := schema.Validate(gojsonschema.NewGoLoader(request.ChartInvocation.Properties))
	if err != nil {
		return nil, err
	}

	// Need to concat errors here
	if !result.Valid() {
		var message bytes.Buffer
		message.WriteString("Properties failed validation:\n")
		for _, err := range result.Errors() {
			message.WriteString(fmt.Sprintf("- %s", err))
		}
		return nil, fmt.Errorf("%s: %s", chartInv.Name, message.String())
	}

	// Fill in defaults (after validation).
	modifiedProperties, err := schema.InsertDefaults(request.ChartInvocation.Properties)
	if err != nil {
		return nil, err
	}

	modifiedResource := *request.ChartInvocation
	modifiedResource.Properties = modifiedProperties

	modifiedRequest := &ServiceRequest{
		ChartInvocation: &modifiedResource,
		Chart:           request.Chart,
	}

	return modifiedRequest, nil
}
