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
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
)

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
	s := cli.New()
	s.PluginsDirectory = "testdata/plugins"
	name := "postrenderer-v1"

	renderer, err := NewPostRendererPlugin(s, name, "ARG1")
	require.NoError(t, err)

	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
	is.NoError(err)
	is.Contains(output.String(), "ARG1")
}

func TestNewPostRenderPluginWithTwoArgsRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the actual Run test uses a basic sed example, so skip this test on windows
		t.Skip("skipping on windows")
	}
	is := assert.New(t)
	s := cli.New()
	s.PluginsDirectory = "testdata/plugins"
	name := "postrenderer-v1"

	renderer, err := NewPostRendererPlugin(s, name, "ARG1", "ARG2")
	require.NoError(t, err)

	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
	is.NoError(err)
	is.Contains(output.String(), "ARG1 ARG2")
}
