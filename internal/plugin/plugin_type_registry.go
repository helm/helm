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
This file contains a "registry" of supported plugin types.

It enables "dyanmic" operations on the go type associated with a given plugin type (see: `helm.sh/helm/v4/internal/plugin/schema` package)

Examples:

```

	// Create a new instance of the output message type for a given plugin type:

	pluginType := "cli/v1" // for example
	ptm, ok := pluginTypesIndex[pluginType]
	if !ok {
		return fmt.Errorf("unknown plugin type %q", pluginType)
	}

	outputMessageType := reflect.Zero(ptm.outputType).Interface()

```

```
// Create a new instance of the config type for a given plugin type

	pluginType := "cli/v1" // for example
	ptm, ok := pluginTypesIndex[pluginType]
	if !ok {
		return nil
	}

	config := reflect.New(ptm.configType).Interface().(Config) // `config` is variable of type `Config`, with

	// validate
	err := config.Validate()
	if err != nil { // handle error }

	// assert to concrete type if needed
	cliConfig := config.(*schema.ConfigCLIV1)

```
*/

package plugin

import (
	"reflect"

	"helm.sh/helm/v4/internal/plugin/schema"
)

type pluginTypeMeta struct {
	pluginType string
	inputType  reflect.Type
	outputType reflect.Type
	configType reflect.Type
}

var pluginTypes = []pluginTypeMeta{
	{
		pluginType: "test/v1",
		inputType:  reflect.TypeFor[schema.InputMessageTestV1](),
		outputType: reflect.TypeFor[schema.OutputMessageTestV1](),
		configType: reflect.TypeFor[schema.ConfigTestV1](),
	},
	{
		pluginType: "cli/v1",
		inputType:  reflect.TypeFor[schema.InputMessageCLIV1](),
		outputType: reflect.TypeFor[schema.OutputMessageCLIV1](),
		configType: reflect.TypeFor[schema.ConfigCLIV1](),
	},
	{
		pluginType: "getter/v1",
		inputType:  reflect.TypeFor[schema.InputMessageGetterV1](),
		outputType: reflect.TypeFor[schema.OutputMessageGetterV1](),
		configType: reflect.TypeFor[schema.ConfigGetterV1](),
	},
	{
		pluginType: "postrenderer/v1",
		inputType:  reflect.TypeFor[schema.InputMessagePostRendererV1](),
		outputType: reflect.TypeFor[schema.OutputMessagePostRendererV1](),
		configType: reflect.TypeFor[schema.ConfigPostRendererV1](),
	},
}

var pluginTypesIndex = func() map[string]*pluginTypeMeta {
	result := make(map[string]*pluginTypeMeta, len(pluginTypes))
	for _, m := range pluginTypes {
		result[m.pluginType] = &m
	}
	return result
}()
