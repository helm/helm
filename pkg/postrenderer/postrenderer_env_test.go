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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
)

func TestNewPostRenderPluginRunPassesSettingsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the plugin is a shell script, so skip this test on windows
		t.Skip("skipping on windows")
	}
	req := require.New(t)

	pluginsDir := t.TempDir()
	pluginDir := filepath.Join(pluginsDir, "postrenderer-env")
	req.NoError(os.Mkdir(pluginDir, 0o755))
	req.NoError(os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(`apiVersion: v1
name: postrenderer-env
version: 1.2.3
type: postrenderer/v1
runtime: subprocess
runtimeConfig:
  platformCommand:
  - command: "${HELM_PLUGIN_DIR}/env.sh"
`), 0o644))
	req.NoError(os.WriteFile(filepath.Join(pluginDir, "env.sh"), []byte(`#!/bin/sh
cat >/dev/null
echo "namespace=${HELM_NAMESPACE} kubecontext=${HELM_KUBECONTEXT} debug=${HELM_DEBUG}"
`), 0o755))

	s := cli.New()
	s.PluginsDirectory = pluginsDir
	s.SetNamespace("test-ns")
	s.KubeContext = "test-context"
	s.Debug = true

	renderer, err := NewPostRendererPlugin(s, "postrenderer-env")
	req.NoError(err)

	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
	req.NoError(err)
	assert.Equal(t, "namespace=test-ns kubecontext=test-context debug=true\n", output.String())
}
