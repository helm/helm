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

package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema/v6"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// ValidateAgainstSchema checks that values does not violate the structure laid out in schema
func ValidateAgainstSchema(chrt *chart.Chart, values map[string]interface{}) error {
	var sb strings.Builder
	if chrt.Schema != nil {
		if err := ValidateAgainstSingleSchema(values, chrt.Schema); err != nil {
			sb.WriteString(fmt.Sprintf("%s:\n", chrt.Name()))
			sb.WriteString(fmt.Sprintf("%s\n", err.Error()))
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

	schema, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("file:///values.schema.json", schema); err != nil {
		return err
	}

	validator, err := compiler.Compile("file:///values.schema.json")
	if err != nil {
		return err
	}

	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return err
	}

	if bytes.Equal(valuesJSON, []byte("null")) {
		valuesJSON = []byte("{}")
	}

	valuesObj, err := jsonschema.UnmarshalJSON(bytes.NewReader(valuesJSON))
	if err != nil {
		return err
	}

	if err = validator.Validate(valuesObj); err != nil {
		var jsonschemaError *jsonschema.ValidationError
		if errors.As(err, &jsonschemaError) {
			// We remove the initial error line as it points to a fake schema file location, and might lead to confusion.
			// We replace the empty location string with `/` to make it more readable.
			cleanErrMessage := strings.Replace(
				strings.Replace(
					jsonschemaError.Error(),
					"jsonschema validation failed with 'file:///values.schema.json#'\n",
					"",
					-1,
				),
				"- at '':",
				"- at '/':",
				-1,
			)

			return errors.New(cleanErrMessage)
		}

		return err
	}

	return nil
}
