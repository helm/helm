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

/*
---
TODO: move this section to public plugin package

Package plugin provides the implementation of the Helm plugin system.

Conceptually, "plugins" enable extending Helm's functionality external to Helm's core codebase. The plugin system allows
code to fetch plugins by type, then invoke the plugin with an input as required by that plugin type. The plugin
returning an output for the caller to consume.

An example of a plugin invocation:
```
d := plugin.Descriptor{
	Type: "example/v1", //
}
plgs, err := plugin.FindPlugins([]string{settings.PluginsDirectory}, d)

for _, plg := range plgs {
	input := &plugin.Input{
		Message: schema.InputMessageExampleV1{ // The type of the input message is defined by the plugin's "type" (example/v1 here)
			...
		},
	}
	output, err := plg.Invoke(context.Background(), input)
	if err != nil {
	    ...
	}

	// consume the output, using type assertion to convert to the expected output type (as defined by the plugin's "type")
	outputMessage, ok := output.Message.(schema.OutputMessageExampleV1)
}

---

Package `plugin` provides the implementation of the Helm plugin system.

Helm plugins are exposed to uses as the "Plugin" type, the basic interface that primarily support the "Invoke" method.

# Plugin Runtimes
Internally, plugins must be implemented by a "runtime" that is responsible for creating the plugin instance, and dispatching the plugin's invocation to the plugin's implementation.
For example:
- forming environment variables and command line args for subprocess execution
- converting input to JSON and invoking a function in a Wasm runtime

Internally, the code structure is:
Runtime.CreatePlugin()
      |
	  | (creates)
	  |
      \---> PluginRuntime
	         |
	         | (implements)
			 v
			 Plugin.Invoke()

# Plugin Types
Each plugin implements a specific functionality, denoted by the plugin's "type" e.g. "getter/v1". The "type" includes a version, in order to allow a given types messaging schema and invocation options to evolve.

Specifically, the plugin's "type" specifies the contract for the input and output messages that are expected to be passed to the plugin, and returned from the plugin. The plugin's "type" also defines the options that can be passed to the plugin when invoking it.

# Metadata
Each plugin must have a `plugin.yaml`, that defines the plugin's metadata. The metadata includes the plugin's name, version, and other information.

For legacy plugins, the type is inferred by which fields are set on the plugin: a downloader plugin is inferred when metadata contains a "downloaders" yaml node, otherwise it is assumed to define a Helm CLI subcommand.

For v1 plugins, the metadata includes explicit apiVersion and type fields. It will also contain type-specific Config, and RuntimeConfig fields.

# Runtime and type cardinality
From a cardinality perspective, this means there a "few" runtimes, and "many" plugins types. It is also expected that the subprocess runtime will not be extended to support extra plugin types, and deprecated in a future version of Helm.

Future ideas that are intended to be implemented include extending the plugin system to support future Wasm standards. Or allowing Helm SDK user's to inject "plugins" that are actually implemented as native go modules. Or even moving Helm's internal functionality e.g. yaml rendering engine to be used as an "in-built" plugin, along side other plugins that may implement other (non-go template) rendering engines.
*/

package plugin
