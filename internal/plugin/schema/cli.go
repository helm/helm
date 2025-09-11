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
)

type InputMessageCLIV1 struct {
	ExtraArgs []string `json:"extraArgs"`
}

type OutputMessageCLIV1 struct {
	Data *bytes.Buffer `json:"data"`
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
