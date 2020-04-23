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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
)

// Values lints a chart's values.yaml file.
//
// This function is deprecated and will be removed in Helm 4.
func Values(linter *support.Linter) {
	ValuesWithOverrides(linter, map[string]interface{}{})
}

// ValuesWithOverrides tests the values.yaml file.
//
// If a schema is present in the chart, values are tested against that. Otherwise,
// they are only tested for well-formedness.
//
// If additional values are supplied, they are coalesced into the values in values.yaml.
func ValuesWithOverrides(linter *support.Linter, values map[string]interface{}) {
	file := "values.yaml"
	vf := filepath.Join(linter.ChartDir, file)
	fileExists := linter.RunLinterRule(support.InfoSev, file, validateValuesFileExistence(vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, file, validateValuesFile(vf, values))
}

func validateValuesFileExistence(valuesPath string) error {
	_, err := os.Stat(valuesPath)
	if err != nil {
		return errors.Errorf("file does not exist")
	}
	return nil
}

func validateValuesFile(valuesPath string, overrides map[string]interface{}) error {
	values, err := chartutil.ReadValuesFile(valuesPath)
	if err != nil {
		return errors.Wrap(err, "unable to parse YAML")
	}

	// Helm 3.0.0 carried over the values linting from Helm 2.x, which only tests the top
	// level values against the top-level expectations. Subchart values are not linted.
	// We could change that. For now, though, we retain that strategy, and thus can
	// coalesce tables (like reuse-values does) instead of doing the full chart
	// CoalesceValues.
	values = chartutil.CoalesceTables(values, overrides)

	ext := filepath.Ext(valuesPath)
	schemaPath := valuesPath[:len(valuesPath)-len(ext)] + ".schema.json"
	schema, err := ioutil.ReadFile(schemaPath)
	if len(schema) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	return chartutil.ValidateAgainstSingleSchema(values, schema)
}
