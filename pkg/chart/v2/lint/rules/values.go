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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
)

// ValuesWithOverrides tests the values.yaml file.
//
// If a schema is present in the chart, values are tested against that. Otherwise,
// they are only tested for well-formedness.
//
// If additional values are supplied, they are coalesced into the values in values.yaml.
func ValuesWithOverrides(linter *support.Linter, valueOverrides map[string]any, skipSchemaValidation bool) {
	file := "values.yaml"
	vf := filepath.Join(linter.ChartDir, file)
	fileExists := linter.RunLinterRule(support.InfoSev, file, validateValuesFileExistence(vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, file, validateValuesFile(vf, valueOverrides, skipSchemaValidation))
	linter.RunLinterRule(support.WarningSev, file, validateValuesFileDuplicateKeys(vf))
}

func validateValuesFileExistence(valuesPath string) error {
	_, err := os.Stat(valuesPath)
	if err != nil {
		return errors.New("file does not exist")
	}
	return nil
}

// validateValuesFileDuplicateKeys reports mapping keys that are defined more than
// once in the same YAML mapping.
//
// Helm loads values.yaml permissively: when a key is repeated the last definition
// silently wins. That is almost always a mistake, so it is flagged here. Syntax
// errors are reported by validateValuesFile, so they are ignored here.
func validateValuesFileDuplicateKeys(valuesPath string) error {
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return nil
	}

	// The yaml.v3 decoder rejects duplicate mapping keys and reports the line
	// numbers of both definitions.
	var values any
	if err := yaml.Unmarshal(data, &values); err != nil {
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			return fmt.Errorf("values contain duplicate keys, the last definition is used: %s", strings.Join(typeErr.Errors, "; "))
		}
	}
	return nil
}

func validateValuesFile(valuesPath string, overrides map[string]any, skipSchemaValidation bool) error {
	values, err := common.ReadValuesFile(valuesPath)
	if err != nil {
		return fmt.Errorf("unable to parse YAML: %w", err)
	}

	// Helm 3.0.0 carried over the values linting from Helm 2.x, which only tests the top
	// level values against the top-level expectations. Subchart values are not linted.
	// We could change that. For now, though, we retain that strategy, and thus can
	// coalesce tables (like reuse-values does) instead of doing the full chart
	// CoalesceValues
	coalescedValues := util.CoalesceTables(make(map[string]any, len(overrides)), overrides)
	coalescedValues = util.CoalesceTables(coalescedValues, values)

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
		return util.ValidateAgainstSingleSchema(coalescedValues, schema)
	}

	return nil
}
