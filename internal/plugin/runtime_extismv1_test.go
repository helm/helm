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
	"context"
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

	cmd := exec.CommandContext(t.Context(), "make", "-C", dir)
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
	assert.NoError(t, rc.Validate(), "expected no error for empty RuntimeConfigExtismV1")
}

func TestRuntimeExtismV1InvokePlugin(t *testing.T) {
	r := RuntimeExtismV1{}

	pr := buildLoadExtismPlugin(t, "testdata/src/extismv1-test")
	require.Equal(t, "test/v1", pr.Metadata.Type)

	p, err := r.CreatePlugin(pr.Dir, &pr.Metadata)

	require.NoError(t, err, "expected no error creating plugin")
	assert.NotNil(t, p, "expected plugin to be created")

	output, err := p.Invoke(t.Context(), &Input{
		Message: schema.InputMessageTestV1{
			Name: "Phippy",
		},
	})
	require.NoError(t, err)

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

func extismTestHostFunction(name string) extism.HostFunction {
	return extism.NewHostFunctionWithStack(name,
		func(context.Context, *extism.CurrentPlugin, []uint64) {},
		[]extism.ValueType{extism.ValueTypeI64},
		nil)
}

func TestBuildHostFunctions(t *testing.T) {
	registry := map[string]extism.HostFunction{
		"helm_first":  extismTestHostFunction("helm_first"),
		"helm_second": extismTestHostFunction("helm_second"),
	}

	tests := []struct {
		name      string
		registry  map[string]extism.HostFunction
		requested []string
		wantNames []string
		wantErr   string
	}{
		{
			name:      "no host functions requested",
			registry:  registry,
			requested: nil,
			wantNames: []string{},
		},
		{
			name:      "one host function requested",
			registry:  registry,
			requested: []string{"helm_first"},
			wantNames: []string{"helm_first"},
		},
		{
			name:      "request order is kept and nothing is prepended",
			registry:  registry,
			requested: []string{"helm_second", "helm_first"},
			wantNames: []string{"helm_second", "helm_first"},
		},
		{
			name:      "the same function may be requested twice",
			registry:  registry,
			requested: []string{"helm_first", "helm_first"},
			wantNames: []string{"helm_first", "helm_first"},
		},
		{
			name:      "unknown function",
			registry:  registry,
			requested: []string{"helm_first", "helm_missing"},
			wantErr:   `plugin requested host function "helm_missing" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &RuntimeConfigExtismV1{HostFunctions: tt.requested}

			result, err := buildHostFunctions(tt.registry, rc)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.Len(t, result, len(tt.wantNames))

			gotNames := make([]string, len(result))
			for i, fn := range result {
				// A zero-value HostFunction has no namespace, which makes
				// extism.NewPlugin fail with "a module name must not be
				// empty", so every entry must be the registered one.
				assert.Equal(t, "extism:host/user", fn.Namespace, "entry %d is a zero-value host function", i)
				gotNames[i] = fn.Name
			}
			assert.Equal(t, tt.wantNames, gotNames)
		})
	}
}

func TestRuntimeExtismV1InvokeWithHostFunctions(t *testing.T) {
	pr := buildLoadExtismPlugin(t, "testdata/src/extismv1-test")

	rc, ok := pr.Metadata.RuntimeConfig.(*RuntimeConfigExtismV1)
	require.True(t, ok, "expected an extism/v1 runtime config, got %T", pr.Metadata.RuntimeConfig)
	rc.HostFunctions = []string{"helm_test_fn", "helm_other_fn"}

	r := RuntimeExtismV1{HostFunctions: map[string]extism.HostFunction{
		"helm_test_fn":  extismTestHostFunction("helm_test_fn"),
		"helm_other_fn": extismTestHostFunction("helm_other_fn"),
	}}

	p, err := r.CreatePlugin(pr.Dir, &pr.Metadata)
	require.NoError(t, err, "expected no error creating plugin")

	// Before the fix the slice handed to extism.NewPlugin held two zero-value
	// host functions ahead of the real ones, and the invocation failed with
	// "failed to create existing plugin: building host module: a module name
	// must not be empty".
	output, err := p.Invoke(t.Context(), &Input{
		Message: schema.InputMessageTestV1{
			Name: "Phippy",
		},
	})
	require.NoError(t, err)

	msg := output.Message.(schema.OutputMessageTestV1)
	assert.Equal(t, "Hello, Phippy! (6)", msg.Greeting)
}
