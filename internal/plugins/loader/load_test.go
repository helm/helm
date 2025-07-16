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

package pluginloader // import "helm.sh/helm/v4/internal/plugins/loader"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugins"
	"helm.sh/helm/v4/internal/plugins/runtimes/subprocess"
)

func TestConvertSubprocess(t *testing.T) {
	sps := []*subprocess.Plugin{
		{
			Metadata: &subprocess.Metadata{Name: "test-plugin"},
		},
	}

	ps := convertSubprocess(sps)

	require.Equal(t, len(sps), len(ps))
	for index := range sps {
		assert.Equal(t, sps[index], ps[index].(*subprocess.Plugin))
	}
}

func TestFindPlugins(t *testing.T) {
	pluginsDirs := []string{"testdata/plugins"}

	findFunc := func(pluginsDir string) ([]plugins.Plugin, error) {
		return []plugins.Plugin{
			&subprocess.Plugin{
				Dir: filepath.Join(pluginsDir, "test-plugin"),
				Metadata: &subprocess.Metadata{
					Name: "test-plugin",
				},
			},
		}, nil
	}

	filterFunc := func(p plugins.Plugin) bool {
		assert.Equal(t, "test-plugin", p.Manifest().Name)
		return true
	}

	ps, err := findPlugins(pluginsDirs, findFunc, filterFunc)
	assert.NoError(t, err)
	require.Len(t, ps, 1)
	assert.Equal(t, "test-plugin", ps[0].Manifest().Name)
}

func TestMakeDescriptorFilter(t *testing.T) {
	descriptor := plugins.PluginDescriptor{
		TypeVersion: "getter/v1",
	}

	filterFunc := makeDescriptorFilter(descriptor)

	ps := []plugins.Plugin{
		&subprocess.Plugin{
			Metadata: &subprocess.Metadata{
				Name: "test-plugin",
				// subprocess plugins classify themselves as "cli" or "downloader" based on presence of Downloaders field
				Downloaders: []subprocess.Downloaders{
					{
						Protocols: []string{"http"},
					},
				},
			},
		},
		&subprocess.Plugin{
			Metadata: &subprocess.Metadata{
				Name: "other-plugin",
			},
		},
	}

	filtered := []plugins.Plugin{}
	for _, p := range ps {
		if filterFunc(p) {
			filtered = append(filtered, p)
		}
	}

	require.Len(t, filtered, 1)
	assert.Equal(t, "test-plugin", filtered[0].Manifest().Name)
}
