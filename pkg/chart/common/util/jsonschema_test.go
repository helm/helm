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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestValidateAgainstSingleSchema(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/test-values.yaml")
	require.NoError(t, err, "Error reading YAML file")

	schema, err := os.ReadFile("./testdata/test-values.schema.json")
	require.NoError(t, err, "Error reading YAML file")
	assert.NoErrorf(t, ValidateAgainstSingleSchema(values, schema), "Error validating Values against Schema")
}

func TestValidateAgainstInvalidSingleSchema(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/test-values.yaml")
	require.NoError(t, err, "Error reading YAML file")

	schema, err := os.ReadFile("./testdata/test-values-invalid.schema.json")
	require.NoError(t, err, "Error reading YAML file")

	expectedErrString := `"file:///values.schema.json#" is not valid against metaschema: jsonschema validation failed with 'https://json-schema.org/draft/2020-12/schema#'
- at '': got number, want boolean or object`
	assert.EqualError(t, ValidateAgainstSingleSchema(values, schema), expectedErrString)
}

func TestValidateAgainstSingleSchemaNegative(t *testing.T) {
	values, err := common.ReadValuesFile("./testdata/test-values-negative.yaml")
	require.NoError(t, err, "Error reading YAML file")

	schema, err := os.ReadFile("./testdata/test-values.schema.json")
	require.NoError(t, err, "Error reading JSON file")

	expectedErrString := `- at '': missing property 'employmentInfo'
- at '/age': minimum: got -5, want 0
`
	assert.EqualError(t, ValidateAgainstSingleSchema(values, schema), expectedErrString)
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

	assert.NoErrorf(t, ValidateAgainstSchema(chrt, vals), "Error validating Values against Schema")
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

	expectedErrString := `subchart:
- at '': missing property 'age'
`
	assert.EqualError(t, ValidateAgainstSchema(chrt, vals), expectedErrString)
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

	assert.NoErrorf(t, ValidateAgainstSchema(chrt, vals), "Error validating Values against Schema")
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

	expectedErrString := `subchart:
- at '/data': no items match contains schema
  - at '/data/0': got number, want string
`
	assert.EqualError(t, ValidateAgainstSchema(chrt, vals), expectedErrString)
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
		require.NoError(t, err, "Expected no error, got")
		require.NotNil(t, result, "Expected result to be non-nil")
	})

	t.Run("HTTP error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		loader := newHTTPURLLoader()
		_, err := loader.Load(server.URL)
		require.Error(t, err, "Expected error for HTTP 404")
		assert.ErrorContains(t, err, "404", "Expected error message to contain '404'")
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
	require.NoErrorf(t, ValidateAgainstSingleSchema(vals, schema), "expected no error when URN unresolved is ignored, got")
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
		require.Nilf(t, recover(), "ValidateAgainstSchema panicked (missing subchart values)")
	}()

	require.NoErrorf(t, ValidateAgainstSchema(chrt, vals), "expected no error when subchart values are missing, got")
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
		require.Nilf(t, recover(), "ValidateAgainstSchema panicked (nil subchart values)")
	}()

	require.NoErrorf(t, ValidateAgainstSchema(chrt, vals), "expected no error when subchart values are nil, got")
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
		require.Nilf(t, recover(), "ValidateAgainstSchema panicked (invalid subchart values type)")
	}()

	// We expect a non-nil error (invalid type), but crucially no panic.
	require.Error(t, ValidateAgainstSchema(chrt, vals), "expected an error when subchart values have invalid type, got nil")
}
