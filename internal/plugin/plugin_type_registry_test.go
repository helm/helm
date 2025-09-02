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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func TestMakeOutputMessage(t *testing.T) {
	ptm := pluginTypesIndex["getter/v1"]
	outputType := reflect.Zero(ptm.outputType).Interface()
	assert.IsType(t, schema.OutputMessageGetterV1{}, outputType)

}

func TestMakeConfig(t *testing.T) {
	ptm := pluginTypesIndex["getter/v1"]
	config := reflect.New(ptm.configType).Interface().(Config)
	assert.IsType(t, &schema.ConfigGetterV1{}, config)
}
