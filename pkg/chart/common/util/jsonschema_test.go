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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestValidateAgainstSingleSchema(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/test-values.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schema, err := os.ReadFile("./testdata/test-values.schema.json")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}

	if err := ValidateAgainstSingleSchema(values, schema); err != nil {
		t.Errorf("Error validating Values against Schema: %s", err)
	}
}

func TestValidateAgainstInvalidSingleSchema(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/test-values.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schema, err := os.ReadFile("./testdata/test-values-invalid.schema.json")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}

	var errString string
	if err := ValidateAgainstSingleSchema(values, schema); err == nil {
		t.Fatalf("Expected an error, but got nil")
	} else {
		errString = err.Error()
	}

	expectedErrString := `"file:///values.schema.json#" is not valid against metaschema: jsonschema validation failed with 'https://json-schema.org/draft/2020-12/schema#'
- at '': got number, want boolean or object`
	if errString != expectedErrString {
		t.Errorf("Error string :\n`%s`\ndoes not match expected\n`%s`", errString, expectedErrString)
	}
}

func TestValidateAgainstSingleSchemaNegative(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/test-values-negative.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schema, err := os.ReadFile("./testdata/test-values.schema.json")
	if err != nil {
		t.Fatalf("Error reading JSON file: %s", err)
	}

	var errString string
	if err := ValidateAgainstSingleSchema(values, schema); err == nil {
		t.Fatalf("Expected an error, but got nil")
	} else {
		errString = err.Error()
	}

	expectedErrString := `- at '': missing property 'employmentInfo'
- at '/age': minimum: got -5, want 0
`
	if errString != expectedErrString {
		t.Errorf("Error string :\n`%s`\ndoes not match expected\n`%s`", errString, expectedErrString)
	}
}

const subchartSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Values",
  "type": "object",
  "properties": {
    "age": {
      "description": "Age",
      "minimum": 0,
      "type": "integer"
    }
  },
  "required": [
    "age"
  ]
}
`

const subchartSchema2020 = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"title": "Values",
	"type": "object",
	"properties": {
		"data": {
			"type": "array",
			"contains": { "type": "string" },
			"unevaluatedItems": { "type": "number" }
		}
	},
	"required": ["data"]
}
`

func TestValidateAgainstSchema(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Schema: subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "chrt",
		},
	}
	chrt.AddDependency(subchart)

	vals := map[string]any{
		"name": "John",
		"subchart": map[string]any{
			"age": 25,
		},
	}

	if err := ValidateAgainstSchema(chrt, vals); err != nil {
		t.Errorf("Error validating Values against Schema: %s", err)
	}
}

func TestValidateAgainstSchemaNegative(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Schema: subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "chrt",
		},
	}
	chrt.AddDependency(subchart)

	vals := map[string]any{
		"name":     "John",
		"subchart": map[string]any{},
	}

	var errString string
	if err := ValidateAgainstSchema(chrt, vals); err == nil {
		t.Fatalf("Expected an error, but got nil")
	} else {
		errString = err.Error()
	}

	expectedValidationError := "missing property 'age'"
	if !strings.Contains(errString, "subchart:") {
		t.Errorf("Error string should contain 'subchart:', got: %s", errString)
	}
	if !strings.Contains(errString, expectedValidationError) {
		t.Errorf("Error string should contain '%s', got: %s", expectedValidationError, errString)
	}
}

func TestValidateAgainstSchema2020(t *testing.T) {
	subchartJSON := []byte(subchartSchema2020)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Schema: subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "chrt",
		},
	}
	chrt.AddDependency(subchart)

	vals := map[string]any{
		"name": "John",
		"subchart": map[string]any{
			"data": []any{"hello", 12},
		},
	}

	if err := ValidateAgainstSchema(chrt, vals); err != nil {
		t.Errorf("Error validating Values against Schema: %s", err)
	}
}

func TestValidateAgainstSchema2020Negative(t *testing.T) {
	subchartJSON := []byte(subchartSchema2020)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Schema: subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "chrt",
		},
	}
	chrt.AddDependency(subchart)

	vals := map[string]any{
		"name": "John",
		"subchart": map[string]any{
			"data": []any{12},
		},
	}

	var errString string
	if err := ValidateAgainstSchema(chrt, vals); err == nil {
		t.Fatalf("Expected an error, but got nil")
	} else {
		errString = err.Error()
	}

	expectedValidationErrors := []string{
		"no items match contains schema",
		"got number, want string",
	}
	if !strings.Contains(errString, "subchart:") {
		t.Errorf("Error string should contain 'subchart:', got: %s", errString)
	}
	for _, expectedErr := range expectedValidationErrors {
		if !strings.Contains(errString, expectedErr) {
			t.Errorf("Error string should contain '%s', got: %s", expectedErr, errString)
		}
	}
}

// TestValidateWithRelativeSchemaReferences tests schema validation with relative $ref paths
// This mimics the behavior of "helm lint ." where the schema is in the current directory
func TestValidateWithRelativeSchemaReferencesCurrentDir(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/current-dir-test/test-values.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schemaPath := "./testdata/current-dir-test/values.schema.json"
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Error reading JSON schema file: %s", err)
	}

	absSchemaPath, err := filepath.Abs(schemaPath)
	if err != nil {
		t.Fatalf("Error getting absolute path: %s", err)
	}

	if err := ValidateAgainstSingleSchemaWithPath(values, schema, absSchemaPath); err != nil {
		t.Errorf("Error validating Values against Schema with relative references: %s", err)
	}
}

// TestValidateWithRelativeSchemaReferencesSubfolder tests schema validation with relative $ref paths
// This mimics the behavior of "helm lint subfolder" where the schema is in a subdirectory
func TestValidateWithRelativeSchemaReferencesSubfolder(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/subdir-test/subfolder/test-values.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	schemaPath := "./testdata/subdir-test/subfolder/values.schema.json"
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Error reading JSON schema file: %s", err)
	}

	absSchemaPath, err := filepath.Abs(schemaPath)
	if err != nil {
		t.Fatalf("Error getting absolute path: %s", err)
	}

	if err := ValidateAgainstSingleSchemaWithPath(values, schema, absSchemaPath); err != nil {
		t.Errorf("Error validating Values against Schema with relative references from subfolder: %s", err)
	}
}

func TestHTTPURLLoader_Load(t *testing.T) {
	// Test successful JSON schema loading
	t.Run("successful load", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"type": "object", "properties": {"name": {"type": "string"}}}`))
		}))
		defer server.Close()

		loader := newHTTPURLLoader()
		result, err := loader.Load(server.URL)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if result == nil {
			t.Fatal("Expected result to be non-nil")
		}
	})

	t.Run("HTTP error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		loader := newHTTPURLLoader()
		_, err := loader.Load(server.URL)
		if err == nil {
			t.Fatal("Expected error for HTTP 404")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("Expected error message to contain '404', got: %v", err)
		}
	})
}

// Test that an unresolved URN $ref is soft-ignored and validation succeeds.
// it mimics the behavior of Helm 3.18.4
func TestValidateAgainstSingleSchema_UnresolvedURN_Ignored(t *testing.T) {
	schema := []byte(`{
        "$schema": "https://json-schema.org/draft-07/schema#",
        "$ref": "urn:example:helm:schemas:v1:helm-schema-validation-conditions:v1/helmSchemaValidation-true"
    }`)
	vals := map[string]any{"any": "value"}
	if err := ValidateAgainstSingleSchema(vals, schema); err != nil {
		t.Fatalf("expected no error when URN unresolved is ignored, got: %v", err)
	}
}

// Non-regression tests for https://github.com/helm/helm/issues/31202
// Ensure ValidateAgainstSchema does not panic when:
// - subchart key is missing
// - subchart value is nil
// - subchart value has an invalid type

func TestValidateAgainstSchema_MissingSubchartValues_NoPanic(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "subchart"},
		Schema:   subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "chrt"},
	}
	chrt.AddDependency(subchart)

	// No "subchart" key present in values
	vals := map[string]any{
		"name": "John",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ValidateAgainstSchema panicked (missing subchart values): %v", r)
		}
	}()

	if err := ValidateAgainstSchema(chrt, vals); err != nil {
		t.Fatalf("expected no error when subchart values are missing, got: %v", err)
	}
}

func TestValidateAgainstSchema_SubchartNil_NoPanic(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "subchart"},
		Schema:   subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "chrt"},
	}
	chrt.AddDependency(subchart)

	// "subchart" key present but nil
	vals := map[string]any{
		"name":     "John",
		"subchart": nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ValidateAgainstSchema panicked (nil subchart values): %v", r)
		}
	}()

	if err := ValidateAgainstSchema(chrt, vals); err != nil {
		t.Fatalf("expected no error when subchart values are nil, got: %v", err)
	}
}

func TestValidateAgainstSchema_InvalidSubchartValuesType_NoPanic(t *testing.T) {
	subchartJSON := []byte(subchartSchema)
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "subchart"},
		Schema:   subchartJSON,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "chrt"},
	}
	chrt.AddDependency(subchart)

	// "subchart" is the wrong type (string instead of map)
	vals := map[string]any{
		"name":     "John",
		"subchart": "oops",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ValidateAgainstSchema panicked (invalid subchart values type): %v", r)
		}
	}()

	// We expect a non-nil error (invalid type), but crucially no panic.
	if err := ValidateAgainstSchema(chrt, vals); err == nil {
		t.Fatalf("expected an error when subchart values have invalid type, got nil")
	}
}

// Test that $ref resolution works for aliased subcharts.
// When a subchart has an alias (e.g., mysql aliased as "database"),
// processDependencyEnabled rewrites Metadata.Name to the alias,
// but the on-disk directory retains the original name (charts/mysql/).
// The schema validator must find the correct directory to resolve $ref.
func TestValidateAgainstSchemaWithPath_AliasedSubchartRef(t *testing.T) {
	tmpDir := t.TempDir()

	// On-disk layout: charts/mysql/ contains the schema files.
	// The directory name is the ORIGINAL chart name, not the alias.
	mysqlDir := filepath.Join(tmpDir, "charts", "mysql")
	if err := os.MkdirAll(mysqlDir, 0o755); err != nil {
		t.Fatal(err)
	}

	baseSchema := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "port": { "type": "integer", "minimum": 1 }
  },
  "required": ["port"]
}`)
	if err := os.WriteFile(filepath.Join(mysqlDir, "base.schema.json"), baseSchema, 0o644); err != nil {
		t.Fatal(err)
	}

	subchartSchemaBytes := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "config": { "$ref": "./base.schema.json" }
  },
  "required": ["config"]
}`)
	if err := os.WriteFile(filepath.Join(mysqlDir, "values.schema.json"), subchartSchemaBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// In-memory chart: Metadata.Name is the ALIAS ("database"),
	// simulating what processDependencyEnabled does after loading.
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "database"},
		Schema:   subchartSchemaBytes,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "testchart"},
	}
	chrt.AddDependency(subchart)

	vals := map[string]any{
		"database": map[string]any{
			"config": map[string]any{
				"port": 3306,
			},
		},
	}

	if err := ValidateAgainstSchemaWithPath(chrt, vals, tmpDir); err != nil {
		t.Errorf("expected no error for valid values with aliased subchart $ref, got: %s", err)
	}
}

// Test that $ref resolution works when multiple aliases point to the same chart.
// Both aliased subcharts should resolve $ref through the single on-disk directory.
func TestValidateAgainstSchemaWithPath_MultipleAliasesSameChart(t *testing.T) {
	tmpDir := t.TempDir()

	mysqlDir := filepath.Join(tmpDir, "charts", "mysql")
	if err := os.MkdirAll(mysqlDir, 0o755); err != nil {
		t.Fatal(err)
	}

	baseSchema := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "port": { "type": "integer", "minimum": 1 }
  },
  "required": ["port"]
}`)
	if err := os.WriteFile(filepath.Join(mysqlDir, "base.schema.json"), baseSchema, 0o644); err != nil {
		t.Fatal(err)
	}

	subchartSchemaBytes := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "config": { "$ref": "./base.schema.json" }
  },
  "required": ["config"]
}`)
	if err := os.WriteFile(filepath.Join(mysqlDir, "values.schema.json"), subchartSchemaBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// Two aliased subcharts from the same original chart
	primary := &chart.Chart{
		Metadata: &chart.Metadata{Name: "primary"},
		Schema:   subchartSchemaBytes,
	}
	replica := &chart.Chart{
		Metadata: &chart.Metadata{Name: "replica"},
		Schema:   subchartSchemaBytes,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "testchart"},
	}
	chrt.AddDependency(primary)
	chrt.AddDependency(replica)

	vals := map[string]any{
		"primary": map[string]any{
			"config": map[string]any{"port": 3306},
		},
		"replica": map[string]any{
			"config": map[string]any{"port": 3307},
		},
	}

	if err := ValidateAgainstSchemaWithPath(chrt, vals, tmpDir); err != nil {
		t.Errorf("expected no error for multiple aliases of same chart, got: %s", err)
	}
}

// Test that validation proceeds gracefully when an aliased subchart has no
// matching directory on disk (e.g., the subchart is an archived .tgz).
// $ref resolution is disabled but main schema validation still works.
func TestValidateAgainstSchemaWithPath_AliasedSubchartNoDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty charts/ directory — no subdirectory matching any name
	if err := os.MkdirAll(filepath.Join(tmpDir, "charts"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Schema without $ref — validates independently
	subchartSchemaBytes := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "port": { "type": "integer" }
  },
  "required": ["port"]
}`)

	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "database"},
		Schema:   subchartSchemaBytes,
	}
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "testchart"},
	}
	chrt.AddDependency(subchart)

	vals := map[string]any{
		"database": map[string]any{
			"port": 5432,
		},
	}

	if err := ValidateAgainstSchemaWithPath(chrt, vals, tmpDir); err != nil {
		t.Errorf("expected no error when aliased subchart dir missing (graceful fallback), got: %s", err)
	}
}
