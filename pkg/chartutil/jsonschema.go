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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/santhosh-tekuri/jsonschema/v5"

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

	var rawInterface interface{}
	if err = yaml.Unmarshal(valuesJSON, &rawInterface); err != nil {
		return err
	}

	schema, err := jsonschema.CompileString("values.schema.json", string(schemaJSON))
	if err != nil {
		return err
	}

	if err = schema.Validate(rawInterface); err != nil {
		var jsonschemaError *jsonschema.ValidationError
		if errors.As(err, &jsonschemaError) {
			return errors.New(processErrorMessage(jsonschemaError))
		}

		return err
	}

	return nil
}

func processErrorMessage(error *jsonschema.ValidationError) string {
	var sb strings.Builder

	if error.KeywordLocation != "" {
		location := error.InstanceLocation
		if location == "" {
			location = "(root)"
		}

		sb.WriteString(fmt.Sprintf("- %s: %s\n", strings.TrimPrefix(location, "/"), error.Message))
	}

	for _, cause := range error.Causes {
		sb.WriteString(processErrorMessage(cause))
	}

	return sb.String()
}
