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
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/internal/test/ensure"
)

var nonExistingValuesFilePath = filepath.Join("/fake/dir", "values.yaml")

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
		t.Errorf("validateValuesFileExistence to return a linter error, got no error")
	}
}

func TestValidateValuesFileWellFormed(t *testing.T) {
	badYaml := `
	not:well[]{}formed
	`
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(badYaml))
	defer os.RemoveAll(tmpdir)
	valfile := filepath.Join(tmpdir, "values.yaml")
	if err := validateValuesFile(valfile, map[string]interface{}{}); err == nil {
		t.Fatal("expected values file to fail parsing")
	}
}

func TestValidateValuesFileSchema(t *testing.T) {
	yaml := "username: admin\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	defer os.RemoveAll(tmpdir)
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	if err := validateValuesFile(valfile, map[string]interface{}{}); err != nil {
		t.Fatalf("Failed validation with %s", err)
	}
}

func TestValidateValuesFileSchemaFailure(t *testing.T) {
	// 1234 is an int, not a string. This should fail.
	yaml := "username: 1234\npassword: swordfish"
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	defer os.RemoveAll(tmpdir)
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")

	err := validateValuesFile(valfile, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected values file to fail parsing")
	}

	assert.Contains(t, err.Error(), "Expected: string, given: integer", "integer should be caught by schema")
}

func TestValidateValuesFileSchemaOverrides(t *testing.T) {
	yaml := "username: admin"
	overrides := map[string]interface{}{
		"password": "swordfish",
	}
	tmpdir := ensure.TempFile(t, "values.yaml", []byte(yaml))
	defer os.RemoveAll(tmpdir)
	createTestingSchema(t, tmpdir)

	valfile := filepath.Join(tmpdir, "values.yaml")
	if err := validateValuesFile(valfile, overrides); err != nil {
		t.Fatalf("Failed validation with %s", err)
	}
}

func TestValidateValuesFile(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		overrides    map[string]interface{}
		errorMessage string
	}{
		{
			name:      "value added",
			yaml:      "username: admin",
			overrides: map[string]interface{}{"password": "swordfish"},
		},
		{
			name:         "value not overridden",
			yaml:         "username: admin\npassword:",
			overrides:    map[string]interface{}{"username": "anotherUser"},
			errorMessage: "Expected: string, given: null",
		},
		{
			name:      "value overridden",
			yaml:      "username: admin\npassword:",
			overrides: map[string]interface{}{"username": "anotherUser", "password": "swordfish"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := ensure.TempFile(t, "values.yaml", []byte(tt.yaml))
			defer os.RemoveAll(tmpdir)
			createTestingSchema(t, tmpdir)

			valfile := filepath.Join(tmpdir, "values.yaml")

			err := validateValuesFile(valfile, tt.overrides)

			switch {
			case err != nil && tt.errorMessage == "":
				t.Errorf("Failed validation with %s", err)
			case err == nil && tt.errorMessage != "":
				t.Error("expected values file to fail parsing")
			case err != nil && tt.errorMessage != "":
				assert.Contains(t, err.Error(), tt.errorMessage, "Failed with unexpected error")
			}
		})
	}
}

func createTestingSchema(t *testing.T, dir string) string {
	t.Helper()
	schemafile := filepath.Join(dir, "values.schema.json")
	if err := ioutil.WriteFile(schemafile, []byte(testSchema), 0700); err != nil {
		t.Fatalf("Failed to write schema to tmpdir: %s", err)
	}
	return schemafile
}
