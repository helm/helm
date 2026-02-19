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
	"bytes"
	"fmt"
	"reflect"

	"go.yaml.in/yaml/v3"
)

// Config represents a plugin type specific configuration
// It is expected to type assert (cast) the Config to its expected underlying type (schema.ConfigCLIV1, schema.ConfigGetterV1, etc).
type Config interface {
	Validate() error
}

func unmarshalConfig(pluginType string, configData map[string]any) (Config, error) {
	pluginTypeMeta, ok := pluginTypesIndex[pluginType]
	if !ok {
		return nil, fmt.Errorf("unknown plugin type %q", pluginType)
	}

	// TODO: Avoid (yaml) serialization/deserialization for type conversion here

	data, err := yaml.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshel config data (plugin type %s): %w", pluginType, err)
	}

	config := reflect.New(pluginTypeMeta.configType)
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(config.Interface()); err != nil {
		return nil, err
	}

	return config.Interface().(Config), nil
}
