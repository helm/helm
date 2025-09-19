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

package plugin // import "helm.sh/helm/v4/internal/plugin"

import (
	"context"
	"io"
	"regexp"
)

const PluginFileName = "plugin.yaml"

// Plugin defines a plugin instance. The client (Helm codebase) facing type that can be used to introspect and invoke a plugin
type Plugin interface {
	// Dir return the plugin directory (as an absolute path) on the filesystem
	Dir() string

	// Metadata describes the plugin's type, version, etc.
	// (This metadata type is the converted and plugin version independented in-memory representation of the plugin.yaml file)
	Metadata() Metadata

	// Invoke takes the given input, and dispatches the contents to plugin instance
	// The input is expected to be a JSON-serializable object, which the plugin will interpret according to its type
	// The plugin is expected to return a JSON-serializable object, which the invoker
	// will interpret according to the plugin's type
	//
	// Invoke can be thought of as a request/response mechanism. Similar to e.g. http.RoundTripper
	//
	// If plugin's execution fails with a non-zero "return code" (this is plugin runtime implementation specific)
	// an InvokeExecError is returned
	Invoke(ctx context.Context, input *Input) (*Output, error)
}

// PluginHook allows plugins to implement hooks that are invoked on plugin management events (install, upgrade, etc)
type PluginHook interface { //nolint:revive
	InvokeHook(event string) error
}

// Input defines the input message and parameters to be passed to the plugin
type Input struct {
	// Message represents the type-elided value to be passed to the plugin.
	// The plugin is expected to interpret the message according to its type
	// The message object must be JSON-serializable
	Message any

	// Optional: Reader to be consumed plugin's "stdin"
	Stdin io.Reader

	// Optional: Writers to consume the plugin's "stdout" and "stderr"
	Stdout, Stderr io.Writer

	// Optional: Env represents the environment as a list of "key=value" strings
	// see os.Environ
	Env []string
}

// Output defines the output message and parameters the passed from the plugin
type Output struct {
	// Message represents the type-elided value returned from the plugin
	// The invoker is expected to interpret the message according to the plugin's type
	// The message object must be JSON-serializable
	Message any
}

// validPluginName is a regular expression that validates plugin names.
//
// Plugin names can only contain the ASCII characters a-z, A-Z, 0-9, ​_​ and ​-.
var validPluginName = regexp.MustCompile("^[A-Za-z0-9_-]+$")
