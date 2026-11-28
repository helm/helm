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

// InputMessageCLIV1 is the message a CLI plugin receives as input.
type InputMessageCLIV1 struct {
	ExtraArgs []string `json:"extraArgs"`
}

// OutputMessageCLIV1 is the message a CLI plugin returns as output.
type OutputMessageCLIV1 struct {
	Data *bytes.Buffer `json:"data"`
}

// MarshalJSON encodes Data as a JSON string. bytes.Buffer has no exported
// fields, so the default encoding would produce `"data":{}` and silently drop
// whatever the plugin wrote, which runtimes that go through encoding/json
// (e.g. extism/v1) rely on to return the plugin output.
func (m OutputMessageCLIV1) MarshalJSON() ([]byte, error) {
	var data string
	if m.Data != nil {
		data = m.Data.String()
	}
	return json.Marshal(struct {
		Data string `json:"data"`
	}{Data: data})
}

// UnmarshalJSON decodes the data string written by MarshalJSON back into a
// *bytes.Buffer, so a Wasm CLI plugin's output reaches Helm instead of an empty
// buffer.
func (m *OutputMessageCLIV1) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	m.Data = bytes.NewBufferString(decoded.Data)
	return nil
}

// ConfigCLIV1 represents the configuration for CLI plugins
type ConfigCLIV1 struct {
	// Usage is the single-line usage text shown in help
	// For recommended syntax, see [spf13/cobra.command.Command] Use field comment:
	// https://pkg.go.dev/github.com/spf13/cobra#Command
	Usage string `yaml:"usage"`
	// ShortHelp is the short description shown in the 'helm help' output
	ShortHelp string `yaml:"shortHelp"`
	// LongHelp is the long message shown in the 'helm help <this-command>' output
	LongHelp string `yaml:"longHelp"`
	// IgnoreFlags ignores any flags passed in from Helm
	IgnoreFlags bool `yaml:"ignoreFlags"`
}

func (c *ConfigCLIV1) Validate() error {
	// Config validation for CLI plugins
	return nil
}
