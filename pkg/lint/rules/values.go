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

package rules

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/lint/support"
)

// ValuesWithOverrides tests the values.yaml file.
//
// If a schema is present in the chart, values are tested against that. Otherwise,
// they are only tested for well-formedness.
//
// If additional values are supplied, they are coalesced into the values in values.yaml.
func ValuesWithOverrides(linter *support.Linter, valueOverrides map[string]interface{}) {
	file := "values.yaml"
	vf := filepath.Join(linter.ChartDir, file)
	fileExists := linter.RunLinterRule(support.InfoSev, file, validateValuesFileExistence(vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, file, validateValuesFile(vf, valueOverrides, false))
}

// ValuesWithOverridesWithSkipSchemaValidation tests the values.yaml file.
//
// If a schema is present in the chart, values are tested against that. Otherwise,
// they are only tested for well-formedness.
//
// If additional values are supplied, they are coalesced into the values in values.yaml.
func ValuesWithOverridesWithSkipSchemaValidation(linter *support.Linter, values map[string]interface{}, skipSchemaValidation bool) {
	file := "values.yaml"
	vf := filepath.Join(linter.ChartDir, file)
	fileExists := linter.RunLinterRule(support.InfoSev, file, validateValuesFileExistence(vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, file, validateValuesFile(vf, values, skipSchemaValidation))
}

func validateValuesFileExistence(valuesPath string) error {
	_, err := os.Stat(valuesPath)
	if err != nil {
		return errors.Errorf("file does not exist")
	}
	return nil
}

func validateValuesFile(valuesPath string, overrides map[string]interface{}, skipSchemaValidation bool) error {
	values, err := chartutil.ReadValuesFile(valuesPath)
	if err != nil {
		return errors.Wrap(err, "unable to parse YAML")
	}

	// Helm 3.0.0 carried over the values linting from Helm 2.x, which only tests the top
	// level values against the top-level expectations. Subchart values are not linted.
	// We could change that. For now, though, we retain that strategy, and thus can
	// coalesce tables (like reuse-values does) instead of doing the full chart
	// CoalesceValues
	coalescedValues := chartutil.CoalesceTables(make(map[string]interface{}, len(overrides)), overrides)
	coalescedValues = chartutil.CoalesceTables(coalescedValues, values)

	ext := filepath.Ext(valuesPath)
	schemaPath := valuesPath[:len(valuesPath)-len(ext)] + ".schema.json"
	schema, err := os.ReadFile(schemaPath)
	if len(schema) == 0 {
		return nil
	}
	if err != nil {
		return err
	}

	if !skipSchemaValidation {
		res := chartutil.ValidateAgainstSingleSchema(coalescedValues, schema)

		return res
	}

	return nil
}
