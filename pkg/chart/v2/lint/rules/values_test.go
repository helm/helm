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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
)

var nonExistingValuesFilePath = filepath.FromSlash("/fake/dir/values.yaml")

const testSchema = `
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "helm values test schema",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "username",
    "password"
  ],
  "properties": {
    "username": {
      "description": "Your username",
      "type": "string"
    },
    "password": {
      "description": "Your password",
      "type": "string"
    }
  }
}
`

func TestValidateValuesYamlNotDirectory(t *testing.T) {
	_ = os.Mkdir(nonExistingValuesFilePath, os.ModePerm)
	defer os.Remove(nonExistingValuesFilePath)
	assert.Error(t, validateValuesFileExistence(nonExistingValuesFilePath), "validateValuesFileExistence to return a linter error, got no error")
}

func TestValidateValuesFileWellFormed(t *testing.T) {
	badYaml := `
	not:well[]{}formed
	`
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(badYaml))
	valfile := filepath.Join(tmpdir, "values.yaml")
	require.Error(t, validateValuesFile(valfile, map[string]any{}, false), "expected values file to fail parsing")
}

func TestValidateValuesFileSchema(t *testing.T) {
	yaml := "username: admin\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	require.NoErrorf(t, validateValuesFile(valfile, map[string]any{}, false), "Failed validation")
}

func TestValidateValuesFileSchemaFailure(t *testing.T) {
	// 1234 is an int, not a string. This should fail.
	yaml := "username: 1234\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	assert.ErrorContains(t, validateValuesFile(valfile, map[string]any{}, false), "- at '/username': got number, want string")
}

func TestValidateValuesFileSchemaFailureButWithSkipSchemaValidation(t *testing.T) {
	// 1234 is an int, not a string. This should fail normally but pass with skipSchemaValidation.
	yaml := "username: 1234\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	require.NoError(t, validateValuesFile(valfile, map[string]any{}, true), "expected values file to pass parsing because of skipSchemaValidation")
}

func TestValidateValuesFileSchemaOverrides(t *testing.T) {
	yaml := "username: admin"
	overrides := map[string]any{
		"password": "swordfish",
	}
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	require.NoErrorf(t, validateValuesFile(valfile, overrides, false), "Failed validation")
}

func TestValidateValuesFile(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		overrides    map[string]any
		errorMessage string
	}{
		{
			name:      "value added",
			yaml:      "username: admin",
			overrides: map[string]any{"password": "swordfish"},
		},
		{
			name:         "value not overridden",
			yaml:         "username: admin\npassword:",
			overrides:    map[string]any{"username": "anotherUser"},
			errorMessage: "- at '/password': got null, want string",
		},
		{
			name:      "value overridden",
			yaml:      "username: admin\npassword:",
			overrides: map[string]any{"username": "anotherUser", "password": "swordfish"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := ensure.TempFile(t, "values.yaml", []byte(tt.yaml))
			createTestingSchema(t, tmpdir)

			valfile := filepath.Join(tmpdir, "values.yaml")

			err := validateValuesFile(valfile, tt.overrides, false)

			if tt.errorMessage == "" {
				require.NoErrorf(t, err, "Failed validation with")
			} else {
				assert.ErrorContains(t, err, tt.errorMessage)
			}
		})
	}
}

func createTestingSchema(t *testing.T, dir string) string {
	t.Helper()
	schemafile := filepath.Join(dir, "values.schema.json")
	require.NoErrorf(t, os.WriteFile(schemafile, []byte(testSchema), 0o700), "Failed to write schema to tmpdir")
	return schemafile
}
