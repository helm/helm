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
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	newvalidator "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/xeipuuv/gojsonschema"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
)

// ValidateAgainstSchema checks that values does not violate the structure laid out in schema
func ValidateAgainstSchema(chrt *chart.Chart, values map[string]interface{}) error {
	var sb strings.Builder
	if chrt.Schema != nil {
		err := ValidateAgainstSingleSchema(values, chrt.Schema)
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s:\n", chrt.Name()))
			sb.WriteString(err.Error())
		}
	}

	// For each dependency, recursively call this function with the coalesced values
	for _, subchart := range chrt.Dependencies() {
		subchartValues := values[subchart.Name()].(map[string]interface{})
		if err := ValidateAgainstSchema(subchart, subchartValues); err != nil {
			sb.WriteString(err.Error())
		}
	}

	if sb.Len() > 0 {
		return errors.New(sb.String())
	}

	return nil
}

// ValidateAgainstSingleSchema checks that values does not violate the structure laid out in this schema
func ValidateAgainstSingleSchema(values Values, schemaJSON []byte) (reterr error) {
	defer func() {
		if r := recover(); r != nil {
			reterr = fmt.Errorf("unable to validate schema: %s", r)
		}
	}()

	valuesData, err := yaml.Marshal(values)
	if err != nil {
		return err
	}
	valuesJSON, err := yaml.YAMLToJSON(valuesData)
	if err != nil {
		return err
	}
	if bytes.Equal(valuesJSON, []byte("null")) {
		valuesJSON = []byte("{}")
	}

	if processed, err := validateUsingNewValidator(valuesJSON, schemaJSON); processed {
		return err
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	valuesLoader := gojsonschema.NewBytesLoader(valuesJSON)

	result, err := gojsonschema.Validate(schemaLoader, valuesLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		var sb strings.Builder
		for _, desc := range result.Errors() {
			sb.WriteString(fmt.Sprintf("- %s\n", desc))
		}
		return errors.New(sb.String())
	}

	return nil
}

// keep the old behaviour for empty $schema or the one that defined in
// https://github.com/xeipuuv/gojsonschema/blob/v1.2.0/draft.go#L46-L62
func validateUsingNewValidator(valuesJSON, schemaJSON []byte) (bool, error) {
	var partialSchema struct {
		Schema string `json:"$schema"`
	}
	_ = json.Unmarshal(schemaJSON, &partialSchema)
	if partialSchema.Schema == "" {
		return false, nil
	}

	url, err := url.Parse(partialSchema.Schema)
	if err != nil {
		return false, nil
	}
	if url.Host == "json-schema.org" {
		switch url.EscapedPath() {
		case
			"/draft-04/schema",
			"/draft-06/schema",
			"/draft-07/schema":
			return false, nil
		}
	}

	schema, err := newvalidator.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return true, err
	}
	values, err := newvalidator.UnmarshalJSON(bytes.NewReader(valuesJSON))
	if err != nil {
		return true, err
	}

	compiler := newvalidator.NewCompiler()
	err = compiler.AddResource("file:///values.schema.json", schema)
	if err != nil {
		return true, err
	}

	validator, err := compiler.Compile("file:///values.schema.json")
	if err != nil {
		return true, err
	}

	return true, validator.Validate(values)
}
