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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	extism "github.com/extism/go-sdk"

	"helm.sh/helm/v4/internal/plugin/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pluginRaw struct {
	Metadata Metadata
	Dir      string
}

func buildLoadExtismPlugin(t *testing.T, dir string) pluginRaw {
	t.Helper()

	pluginFile := filepath.Join(dir, PluginFileName)

	metadataData, err := os.ReadFile(pluginFile)
	require.NoError(t, err)

	m, err := loadMetadata(metadataData)
	require.NoError(t, err)
	require.Equal(t, "extism/v1", m.Runtime, "expected plugin runtime to be extism/v1")

	cmd := exec.Command("make", "-C", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "failed to build plugin in %q", dir)

	return pluginRaw{
		Metadata: *m,
		Dir:      dir,
	}
}

func TestRuntimeConfigExtismV1Validate(t *testing.T) {
	rc := RuntimeConfigExtismV1{}
	err := rc.Validate()
	assert.NoError(t, err, "expected no error for empty RuntimeConfigExtismV1")
}

func TestRuntimeExtismV1InvokePlugin(t *testing.T) {
	r := RuntimeExtismV1{}

	pr := buildLoadExtismPlugin(t, "testdata/src/extismv1-test")
	require.Equal(t, "test/v1", pr.Metadata.Type)

	p, err := r.CreatePlugin(pr.Dir, &pr.Metadata)

	assert.NoError(t, err, "expected no error creating plugin")
	assert.NotNil(t, p, "expected plugin to be created")

	output, err := p.Invoke(t.Context(), &Input{
		Message: schema.InputMessageTestV1{
			Name: "Phippy",
		},
	})
	require.Nil(t, err)

	msg := output.Message.(schema.OutputMessageTestV1)
	assert.Equal(t, "Hello, Phippy! (6)", msg.Greeting)
}

func TestBuildManifest(t *testing.T) {
	rc := &RuntimeConfigExtismV1{
		Memory: RuntimeConfigExtismV1Memory{
			MaxPages:             8,
			MaxHTTPResponseBytes: 81920,
			MaxVarBytes:          8192,
		},
		FileSystem: RuntimeConfigExtismV1FileSystem{
			CreateTempDir: true,
		},
		Config:       map[string]string{"CONFIG_KEY": "config_value"},
		AllowedHosts: []string{"example.com", "api.example.com"},
		Timeout:      5000,
	}

	expected := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: "/path/to/plugin/plugin.wasm",
				Name: "/path/to/plugin/plugin.wasm",
			},
		},
		Memory: &extism.ManifestMemory{
			MaxPages:             8,
			MaxHttpResponseBytes: 81920,
			MaxVarBytes:          8192,
		},
		Config:       map[string]string{"CONFIG_KEY": "config_value"},
		AllowedHosts: []string{"example.com", "api.example.com"},
		AllowedPaths: map[string]string{"/tmp/foo": "/tmp"},
		Timeout:      5000,
	}

	manifest, err := buildManifest("/path/to/plugin", "/tmp/foo", rc)
	require.NoError(t, err)
	assert.Equal(t, expected, manifest)
}
