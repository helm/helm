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

package schema

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The JSON round trip is what the extism/v1 runtime does with a plugin's
// output message, so the buffer must survive it instead of becoming "{}".
func TestOutputMessageCLIV1JSONRoundTrip(t *testing.T) {
	encoded, err := json.Marshal(OutputMessageCLIV1{Data: bytes.NewBufferString("hello plugin\n")})
	require.NoError(t, err)
	assert.JSONEq(t, `{"data":"hello plugin\n"}`, string(encoded))

	var decoded OutputMessageCLIV1
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.NotNil(t, decoded.Data)
	assert.Equal(t, "hello plugin\n", decoded.Data.String())
}

func TestOutputMessageCLIV1UnmarshalPluginOutput(t *testing.T) {
	var decoded OutputMessageCLIV1
	require.NoError(t, json.Unmarshal([]byte(`{"data":"from the plugin"}`), &decoded))
	require.NotNil(t, decoded.Data)
	assert.Equal(t, "from the plugin", decoded.Data.String())
}
