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

package v2_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	release "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/internal/release/v2/sequence"
)

func TestReleasePlanJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := &release.Release{
		Name: "example",
		Plan: &sequence.PlanRecord{Version: 1},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"example","plan":{"version":1,"levels":null,"batches":null}}`, string(data))

	var decoded release.Release
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Plan, decoded.Plan)
}

func TestReleasePlanOmittedWhenNil(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(&release.Release{Name: "example"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"example"}`, string(data))
}
