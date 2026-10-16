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

package postrenderer

import (
	"bytes"
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/schema"
	"helm.sh/helm/v4/pkg/cli"
)

// nilManifestPlugin simulates a plugin.Plugin whose Invoke() returns a
// well-typed OutputMessagePostRendererV1 with a nil Manifests field. This is
// exactly what internal/plugin's ExtismV1PluginRuntime.Invoke produces (via
// json.Unmarshal into a freshly reflect.New'd struct) when a wasm/extism
// post-renderer plugin's JSON output omits the "manifests" key or sets it to
// null -- e.g. because the plugin errored internally but still exited 0, or
// is simply buggy/malformed. No handcrafted wasm binary is required to prove
// the bug: the defect is in postRendererPlugin.Run's handling of the output
// message, not in how a specific Runtime constructs it.
type nilManifestPlugin struct{}

func (nilManifestPlugin) Dir() string { return "" }
func (nilManifestPlugin) Metadata() plugin.Metadata {
	return plugin.Metadata{Name: "nil-manifest-plugin", Type: "postrenderer/v1"}
}

func (nilManifestPlugin) Invoke(_ context.Context, _ *plugin.Input) (*plugin.Output, error) {
	return &plugin.Output{
		Message: schema.OutputMessagePostRendererV1{Manifests: nil},
	}, nil
}

func TestPostRendererPluginRunWithNilManifestsDoesNotPanic(t *testing.T) {
	r := &postRendererPlugin{plugin: nilManifestPlugin{}}

	_, err := r.Run(bytes.NewBufferString("apiVersion: v1\nkind: ConfigMap\n"))
	require.Error(t, err, "a plugin returning nil Manifests should produce a clean error, not a panic")
}

func TestNewPostRenderPluginRunWithNoOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the actual Run test uses a basic sed example, so skip this test on windows
		t.Skip("skipping on windows")
	}
	is := assert.New(t)
	s := cli.New()
	s.PluginsDirectory = "testdata/plugins"
	name := "postrenderer-v1"

	renderer, err := NewPostRendererPlugin(s, name, "")
	require.NoError(t, err)

	_, err = renderer.Run(bytes.NewBufferString(""))
	is.Error(err)
}

func TestNewPostRenderPluginWithOneArgsRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the actual Run test uses a basic sed example, so skip this test on windows
		t.Skip("skipping on windows")
	}
	is := assert.New(t)
	req := require.New(t)
	s := cli.New()
	s.PluginsDirectory = "testdata/plugins"
	name := "postrenderer-v1"

	renderer, err := NewPostRendererPlugin(s, name, "ARG1")
	req.NoError(err)

	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
	req.NoError(err)
	is.Contains(output.String(), "ARG1")
}

func TestNewPostRenderPluginWithTwoArgsRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the actual Run test uses a basic sed example, so skip this test on windows
		t.Skip("skipping on windows")
	}
	is := assert.New(t)
	req := require.New(t)
	s := cli.New()
	s.PluginsDirectory = "testdata/plugins"
	name := "postrenderer-v1"

	renderer, err := NewPostRendererPlugin(s, name, "ARG1", "ARG2")
	req.NoError(err)

	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
	req.NoError(err)
	is.Contains(output.String(), "ARG1 ARG2")
}
