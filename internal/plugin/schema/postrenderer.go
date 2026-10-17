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
)

// InputMessagePostRendererV1 implements Input.Message
type InputMessagePostRendererV1 struct {
	Manifests *bytes.Buffer `json:"manifests"`
	// from CLI --post-renderer-args
	ExtraArgs []string `json:"extraArgs"`
}

// MarshalJSON encodes Manifests as a JSON string. bytes.Buffer has no
// exported fields, so the default encoding would produce "manifests":{}
// and silently drop the rendered manifests, which runtimes that go
// through encoding/json (e.g. extism/v1) rely on to reach the plugin.
func (m InputMessagePostRendererV1) MarshalJSON() ([]byte, error) {
	var manifests string
	if m.Manifests != nil {
		manifests = m.Manifests.String()
	}
	return json.Marshal(struct {
		Manifests string   `json:"manifests"`
		ExtraArgs []string `json:"extraArgs"`
	}{
		Manifests: manifests,
		ExtraArgs: m.ExtraArgs,
	})
}

// UnmarshalJSON decodes the manifests string written by MarshalJSON back
// into a *bytes.Buffer.
func (m *InputMessagePostRendererV1) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Manifests string   `json:"manifests"`
		ExtraArgs []string `json:"extraArgs"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	m.Manifests = bytes.NewBufferString(decoded.Manifests)
	m.ExtraArgs = decoded.ExtraArgs
	return nil
}

type OutputMessagePostRendererV1 struct {
	Manifests *bytes.Buffer `json:"manifests"`
}

// MarshalJSON encodes Manifests as a JSON string; see
// InputMessagePostRendererV1.MarshalJSON for why.
func (m OutputMessagePostRendererV1) MarshalJSON() ([]byte, error) {
	var manifests string
	if m.Manifests != nil {
		manifests = m.Manifests.String()
	}
	return json.Marshal(struct {
		Manifests string `json:"manifests"`
	}{
		Manifests: manifests,
	})
}

// UnmarshalJSON decodes the manifests string written by MarshalJSON back
// into a *bytes.Buffer.
func (m *OutputMessagePostRendererV1) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Manifests string `json:"manifests"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	m.Manifests = bytes.NewBufferString(decoded.Manifests)
	return nil
}

type ConfigPostRendererV1 struct{}

func (c *ConfigPostRendererV1) Validate() error {
	return nil
}
