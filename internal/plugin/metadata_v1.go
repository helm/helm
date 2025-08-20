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
)

// MetadataV1 is the APIVersion V1 plugin.yaml format
type MetadataV1 struct {
	// APIVersion specifies the plugin API version
	APIVersion string `yaml:"apiVersion"`

	// Name is the name of the plugin
	Name string `yaml:"name"`

	// Type of plugin (eg, cli/v1, getter/v1, postrenderer/v1)
	Type string `yaml:"type"`

	// Runtime specifies the runtime type (subprocess, wasm)
	Runtime string `yaml:"runtime"`

	// Version is a SemVer 2 version of the plugin.
	Version string `yaml:"version"`

	// SourceURL is the URL where this plugin can be found
	SourceURL string `yaml:"sourceURL,omitempty"`

	// Config contains the type-specific configuration for this plugin
	Config map[string]any `yaml:"config"`

	// RuntimeConfig contains the runtime-specific configuration
	RuntimeConfig map[string]any `yaml:"runtimeConfig"`
}

func (m *MetadataV1) Validate() error {
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin `name`")
	}

	if m.APIVersion != "v1" {
		return fmt.Errorf("invalid `apiVersion`: %q", m.APIVersion)
	}

	if m.Type == "" {
		return fmt.Errorf("`type` missing")
	}

	if m.Runtime == "" {
		return fmt.Errorf("`runtime` missing")
	}

	return nil
}
