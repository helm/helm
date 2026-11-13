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

func TestInputMessagePostRendererV1JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		manifests *bytes.Buffer
		extraArgs []string
	}{
		{
			name:      "multi-line manifests with extra args",
			manifests: bytes.NewBufferString("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"),
			extraArgs: []string{"--foo", "bar"},
		},
		{
			name:      "empty manifests",
			manifests: bytes.NewBufferString(""),
			extraArgs: nil,
		},
		{
			name:      "nil manifests",
			manifests: nil,
			extraArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := InputMessagePostRendererV1{
				Manifests: tt.manifests,
				ExtraArgs: tt.extraArgs,
			}

			data, err := json.Marshal(in)
			require.NoError(t, err)

			// json.Marshal on a *bytes.Buffer field with no custom
			// MarshalJSON produces "manifests":{}, silently dropping the
			// rendered manifests. Guard against that regressing.
			assert.NotContains(t, string(data), `"manifests":{}`)

			var out InputMessagePostRendererV1
			require.NoError(t, json.Unmarshal(data, &out))

			wantManifests := ""
			if tt.manifests != nil {
				wantManifests = tt.manifests.String()
			}
			require.NotNil(t, out.Manifests)
			assert.Equal(t, wantManifests, out.Manifests.String())
			assert.Equal(t, tt.extraArgs, out.ExtraArgs)
		})
	}
}

func TestOutputMessagePostRendererV1JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		manifests *bytes.Buffer
	}{
		{
			name:      "multi-line manifests",
			manifests: bytes.NewBufferString("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"),
		},
		{
			name:      "empty manifests",
			manifests: bytes.NewBufferString(""),
		},
		{
			name:      "nil manifests",
			manifests: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := OutputMessagePostRendererV1{Manifests: tt.manifests}

			data, err := json.Marshal(out)
			require.NoError(t, err)
			assert.NotContains(t, string(data), `"manifests":{}`)

			var decoded OutputMessagePostRendererV1
			require.NoError(t, json.Unmarshal(data, &decoded))

			wantManifests := ""
			if tt.manifests != nil {
				wantManifests = tt.manifests.String()
			}
			require.NotNil(t, decoded.Manifests)
			assert.Equal(t, wantManifests, decoded.Manifests.String())
		})
	}
}
