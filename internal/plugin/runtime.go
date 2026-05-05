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
	"strings"

	"go.yaml.in/yaml/v3"
)

// Runtime represents a plugin runtime (subprocess, extism, etc) ie. how a plugin should be executed
// Runtime is responsible for instantiating plugins that implement the runtime
// TODO: could call this something more like "PluginRuntimeCreator"?
type Runtime interface {
	// CreatePlugin creates a plugin instance from the given metadata
	CreatePlugin(pluginDir string, metadata *Metadata) (Plugin, error)

	// TODO: move config unmarshalling to the runtime?
	// UnmarshalConfig(runtimeConfigRaw map[string]any) (RuntimeConfig, error)
}

// RuntimeConfig represents the assertable type for a plugin's runtime configuration.
// It is expected to type assert (cast) the a RuntimeConfig to its expected type
type RuntimeConfig interface {
	Validate() error
}

func remarshalRuntimeConfig[T RuntimeConfig](runtimeData map[string]any) (RuntimeConfig, error) {
	data, err := yaml.Marshal(runtimeData)
	if err != nil {
		return nil, err
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// ParseEnv takes a list of "KEY=value" environment variable strings
// and transforms the result into a map[KEY]=value
//
// - empty input strings are ignored
// - input strings with no value are stored as empty strings
// - duplicate keys overwrite earlier values
func ParseEnv(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) > 0 && parts[0] != "" {
			key := parts[0]
			var value string
			if len(parts) > 1 {
				value = parts[1]
			}
			result[key] = value
		}
	}
	return result
}

// FormatEnv takes a map[KEY]=value and transforms it into
// a list of "KEY=value" environment variable strings
func FormatEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for key, value := range env {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}
	return result
}
