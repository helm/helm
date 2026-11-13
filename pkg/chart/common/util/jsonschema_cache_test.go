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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
)

func TestValidateAgainstSingleSchemaCachesCompilation(t *testing.T) {
	schema, err := os.ReadFile("./testdata/test-values.schema.json")
	require.NoError(t, err, "Error reading JSON file")

	first, err := compileSchema(schema)
	require.NoError(t, err)
	second, err := compileSchema(schema)
	require.NoError(t, err)
	assert.Same(t, first, second, "expected the compiled schema to be reused from the cache")

	other := []byte(`{"type": "object"}`)
	third, err := compileSchema(other)
	require.NoError(t, err)
	assert.NotSame(t, first, third, "expected a different schema to compile separately")

	// The cached schema must still validate values correctly.
	values, err := common.ReadValuesFile("./testdata/test-values.yaml")
	require.NoError(t, err, "Error reading YAML file")
	assert.NoError(t, ValidateAgainstSingleSchema(values, schema))
}

func BenchmarkValidateAgainstSingleSchema(b *testing.B) {
	values, err := common.ReadValuesFile("./testdata/test-values.yaml")
	require.NoError(b, err, "Error reading YAML file")

	schema, err := os.ReadFile("./testdata/test-values.schema.json")
	require.NoError(b, err, "Error reading JSON file")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateAgainstSingleSchema(values, schema)
	}
}

// Regression test for https://github.com/helm/helm/pull/32740: a schema
// whose root document only $refs an external file must pick up changes to
// that file on repeated validation. Caching the compiled schema on the
// root bytes alone keeps validating against the stale external content.
func TestValidateAgainstSingleSchema_ExternalFileRefRefresh(t *testing.T) {
	dir := t.TempDir()
	refPath := filepath.Join(dir, "ref.json")
	require.NoError(t, os.WriteFile(refPath, []byte(`{"type": "object", "properties": {"n": {"minimum": 0}}}`), 0644))

	root := []byte(fmt.Sprintf(`{"$ref": "file://%s"}`, refPath))
	values := common.Values{"n": 5}

	require.NoError(t, ValidateAgainstSingleSchema(values, root))

	require.NoError(t, os.WriteFile(refPath, []byte(`{"type": "object", "properties": {"n": {"minimum": 10}}}`), 0644))
	err := ValidateAgainstSingleSchema(values, root)
	require.Error(t, err, "expected the updated external schema (minimum 10) to reject n=5")
	assert.Contains(t, err.Error(), "minimum")
}

// Same regression via a URN $ref: when the URN resolver starts supplying
// the referenced schema, repeated validation must use it instead of the
// permissive fallback compiled on the first call.
func TestValidateAgainstSingleSchema_ExternalURNRefRefresh(t *testing.T) {
	urn := fmt.Sprintf("urn:example:test:external-ref-cache:%d", time.Now().UnixNano())
	available := false
	origResolver := URNResolver
	URNResolver = func(u string) (any, error) {
		if u != urn || !available {
			return nil, fmt.Errorf("URN not resolved: %s", u)
		}
		return jsonschema.UnmarshalJSON(strings.NewReader(`{"type": "object", "properties": {"n": {"minimum": 10}}}`))
	}
	t.Cleanup(func() { URNResolver = origResolver })

	root := []byte(fmt.Sprintf(`{"$ref": %q}`, urn))
	values := common.Values{"n": 5}

	// Backing data unavailable: the unresolved URN is soft-ignored.
	require.NoError(t, ValidateAgainstSingleSchema(values, root))

	available = true
	err := ValidateAgainstSingleSchema(values, root)
	require.Error(t, err, "expected the now-resolved URN schema (minimum 10) to reject n=5")
	assert.Contains(t, err.Error(), "minimum")
}
