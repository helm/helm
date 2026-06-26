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

package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseSequencingInfoBackwardCompatibility(t *testing.T) {
	release := Release{Name: "demo"}

	data, err := json.Marshal(&release)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sequencing_info")

	var decoded Release
	err = json.Unmarshal([]byte(`{"name":"demo"}`), &decoded)
	require.NoError(t, err)
	assert.Nil(t, decoded.SequencingInfo)
}

func TestReleaseSequencingInfoRoundTrip(t *testing.T) {
	release := Release{
		Name: "demo",
		SequencingInfo: &SequencingInfo{
			Enabled:  true,
			Strategy: "ordered",
		},
	}

	data, err := json.Marshal(&release)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"sequencing_info"`)

	var decoded Release
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.NotNil(t, decoded.SequencingInfo)
	assert.True(t, decoded.SequencingInfo.Enabled)
	assert.Equal(t, "ordered", decoded.SequencingInfo.Strategy)
}
