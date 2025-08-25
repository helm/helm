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

package plugin

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Config interface defines the methods that all plugin type configurations must implement
type Config interface {
	Validate() error
}

// ConfigCLI represents the configuration for CLI plugins
type ConfigCLI struct {
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

// ConfigGetter represents the configuration for download plugins
type ConfigGetter struct {
	// Protocols are the list of URL schemes supported by this downloader
	Protocols []string `yaml:"protocols"`
}

func (c *ConfigCLI) GetType() string    { return "cli/v1" }
func (c *ConfigGetter) GetType() string { return "getter/v1" }

func (c *ConfigCLI) Validate() error {
	// Config validation for CLI plugins
	return nil
}

func (c *ConfigGetter) Validate() error {
	if len(c.Protocols) == 0 {
		return fmt.Errorf("getter has no protocols")
	}
	for i, protocol := range c.Protocols {
		if protocol == "" {
			return fmt.Errorf("getter has empty protocol at index %d", i)
		}
	}
	return nil
}

func remarshalConfig[T Config](configData map[string]any) (Config, error) {
	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}
