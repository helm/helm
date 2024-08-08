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
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"
	cueyaml "cuelang.org/go/encoding/yaml"
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
			sb.WriteString(errors.Details(err, nil))
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
func ValidateAgainstSingleSchema(values Values, schema []byte) (reterr error) {
	defer func() {
		if r := recover(); r != nil {
			reterr = fmt.Errorf("unable to validate schema: %s", r)
		}
	}()

	// Create a new CUE context
	ctx := cuecontext.New()

	valuesData, err := yaml.Marshal(values)
	if err != nil {
		return err
	}

	cueValuesData, err := cueyaml.Extract("values.yaml", valuesData)
	if err != nil {
		return err
	}

	cueValuesDataExpr := ctx.BuildFile(cueValuesData)
	if err := cueValuesDataExpr.Err(); err != nil {
		return err
	}

	// JSONschema Processing with CUE
	jsonSchema, err := json.Extract("values.schema.json", schema)
	if err != nil {
		return err
	}

	cueJSONSchemaExpr := ctx.BuildExpr(jsonSchema)
	if err := cueJSONSchemaExpr.Err(); err != nil {
		return err
	}

	schemaAst, err := jsonschema.Extract(cueJSONSchemaExpr, &jsonschema.Config{
		Strict: false,
	})
	if err != nil {
		return err
	}

	cueSchema := ctx.BuildFile(schemaAst)
	result := cueSchema.Unify(cueValuesDataExpr)

	if err := result.Validate(cue.Concrete(true)); err != nil {
		return err
	}

	return nil
}
