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
	"testing"

	"github.com/stretchr/testify/assert"
	yamlv2 "go.yaml.in/yaml/v2"

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

	err := validateValuesFileExistence(nonExistingValuesFilePath)
	if err == nil {
		t.Error("validateValuesFileExistence to return a linter error, got no error")
	}
}

func TestValidateValuesFileWellFormed(t *testing.T) {
	badYaml := `
	not:well[]{}formed
	`
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(badYaml))
	valfile := filepath.Join(tmpdir, "values.yaml")
	if err := validateValuesFile(valfile, map[string]any{}, false); err == nil {
		t.Fatal("expected values file to fail parsing")
	}
}

func TestValidateValuesFileSchema(t *testing.T) {
	yaml := "username: admin\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	if err := validateValuesFile(valfile, map[string]any{}, false); err != nil {
		t.Fatalf("Failed validation with %s", err)
	}
}

func TestValidateValuesFileSchemaFailure(t *testing.T) {
	// 1234 is an int, not a string. This should fail.
	yaml := "username: 1234\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")

	err := validateValuesFile(valfile, map[string]any{}, false)
	assert.ErrorContains(t, err, "- at '/username': got number, want string")
}

func TestValidateValuesFileSchemaFailureButWithSkipSchemaValidation(t *testing.T) {
	// 1234 is an int, not a string. This should fail normally but pass with skipSchemaValidation.
	yaml := "username: 1234\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")

	err := validateValuesFile(valfile, map[string]any{}, true)
	if err != nil {
		t.Fatal("expected values file to pass parsing because of skipSchemaValidation")
	}
}

func TestValidateValuesFileSchemaOverrides(t *testing.T) {
	yaml := "username: admin"
	overrides := map[string]any{
		"password": "swordfish",
	}
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	if err := validateValuesFile(valfile, overrides, false); err != nil {
		t.Fatalf("Failed validation with %s", err)
	}
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

			switch {
			case err != nil && tt.errorMessage == "":
				t.Errorf("Failed validation with %s", err)
			case err == nil && tt.errorMessage != "":
				t.Error("expected values file to fail parsing")
			case err != nil && tt.errorMessage != "":
				assert.ErrorContains(t, err, tt.errorMessage, "Failed with unexpected error")
			}
		})
	}
}

func createTestingSchema(t *testing.T, dir string) string {
	t.Helper()
	schemafile := filepath.Join(dir, "values.schema.json")
	if err := os.WriteFile(schemafile, []byte(testSchema), 0o700); err != nil {
		t.Fatalf("Failed to write schema to tmpdir: %s", err)
	}
	return schemafile
}

func TestIsDuplicateKeyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "duplicate key error",
			err:      &yamlv2.TypeError{Errors: []string{`line 2: key "key" already set in map`}},
			expected: true,
		},
		{
			name:     "wrapped duplicate key error",
			err:      fmt.Errorf("error converting YAML to JSON: %w", &yamlv2.TypeError{Errors: []string{`line 2: key "key" already set in map`}}),
			expected: true,
		},
		{
			name:     "non-duplicate key error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "type error but not duplicate-key related",
			err:      &yamlv2.TypeError{Errors: []string{"cannot unmarshal !!seq into map[string]interface {}"}},
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDuplicateKeyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateValuesFileDuplicateKeys(t *testing.T) {
	duplicateYaml := `key: value1
key: value2
`
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(duplicateYaml))
	valfile := filepath.Join(tmpdir, "values.yaml")

	err := validateValuesFile(valfile, map[string]any{}, false)
	if err == nil {
		t.Fatal("expected values file with duplicate keys to fail parsing")
	}
	assert.Contains(t, err.Error(), "contains duplicate keys")
}
