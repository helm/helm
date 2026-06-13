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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func TestUnmarshaConfig(t *testing.T) {
	// Test unmarshalling a CLI plugin config
	{
		config, err := unmarshalConfig("cli/v1", map[string]any{
			"usage":       "usage string",
			"shortHelp":   "short help string",
			"longHelp":    "long help string",
			"ignoreFlags": true,
		})
		require.NoError(t, err)

		require.IsType(t, &schema.ConfigCLIV1{}, config)
		assert.Equal(t, schema.ConfigCLIV1{
			Usage:       "usage string",
			ShortHelp:   "short help string",
			LongHelp:    "long help string",
			IgnoreFlags: true,
		}, *(config.(*schema.ConfigCLIV1)))
	}

	// Test unmarshalling invalid config data
	{
		config, err := unmarshalConfig("cli/v1", map[string]any{
			"invalid field": "foo",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field not found")
		assert.Nil(t, config)
	}
}
