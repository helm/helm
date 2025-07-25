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

package plugins

import (
	"context"
)

const PluginFileName = "plugin.yaml"

type PluginDescriptor struct {
	TypeVersion string
}

// plugin.yaml definition
type Manifest struct {
	// APIVersion of the plugin manifest document
	// Currently: 'plugins.helm.sh/v1alpha1'
	APIVersion string `json:"apiVersion"`

	// Author defined name, version and description of the plugin
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`

	// Type/version of the plugin: 'getter/v1', 'postrenderer/v1', 'cli/v1', etc
	// Describing the situation the plugin is expected to be invoked, and the correspondng message type/version used to invoke
	TypeVersion string `json:"typeVersion"`

	// Runtime used to execute the plugin
	// subprocess, extism/v1, etc
	RuntimeClass string `json:"runtimeClass"`

	// Additional config associated with the plugin kind: e.g. downloader URI schemes
	// (Config is intepreted by the plugin invoker)
	Config map[string]any `json:"config,omitempty"`
}

// Input defined the input message and parameters to be passed to the plugin
type Input struct {
	// Message represents the type-elided value to be passed to the plugin
	// The plugin is expected to interpret the message according to its type/version
	// The message object must be JSON-serializable
	Message any
}

// Input defined the output message and parameters the passed from the plugin
type Output struct {
	// Message represents the type-elided value passed from the plugin
	// The invoker is expected to interpret the message according to the plugins type/version
	Message any
}

// Plugin defines the "invokable" interface for a plugin, as well a getter for the plugin's describing manifest
// The invoke method can be thought of request/response message passing between the plugin invoker and the plugin itself
type Plugin interface {
	Manifest() Manifest
	Invoke(ctx context.Context, input *Input) (*Output, error)
}
