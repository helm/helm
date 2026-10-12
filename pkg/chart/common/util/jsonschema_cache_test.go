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
	"os"
	"testing"

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
