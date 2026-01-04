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

package getter

import (
	"context"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/schema"

	"helm.sh/helm/v4/pkg/cli"
)

func TestCollectPlugins(t *testing.T) {
	env := cli.New()
	env.PluginsDirectory = pluginDir

	p, err := collectGetterPlugins(env)
	if err != nil {
		t.Fatal(err)
	}

	if len(p) != 2 {
		t.Errorf("Expected 2 plugins, got %d: %v", len(p), p)
	}

	if _, err := p.ByScheme("test2"); err != nil {
		t.Error(err)
	}

	if _, err := p.ByScheme("test"); err != nil {
		t.Error(err)
	}

	if _, err := p.ByScheme("nosuchthing"); err == nil {
		t.Fatal("did not expect protocol handler for nosuchthing")
	}
}

func TestConvertOptions(t *testing.T) {
	opts := convertOptions(
		[]Option{
			WithURL("example://foo"),
			WithAcceptHeader("Accept-Header"),
			WithBasicAuth("username", "password"),
			WithPassCredentialsAll(true),
			WithUserAgent("User-agent"),
			WithInsecureSkipVerifyTLS(true),
			WithTLSClientConfig("certFile.pem", "keyFile.pem", "caFile.pem"),
			WithPlainHTTP(true),
			WithTimeout(10),
			WithTagName("1.2.3"),
			WithUntar(),
		},
		[]Option{
			WithTimeout(20),
		},
	)

	expected := schema.GetterOptionsV1{
		URL:                   "example://foo",
		CertFile:              "certFile.pem",
		KeyFile:               "keyFile.pem",
		CAFile:                "caFile.pem",
		UNTar:                 true,
		Timeout:               20,
		InsecureSkipVerifyTLS: true,
		PlainHTTP:             true,
		AcceptHeader:          "Accept-Header",
		Username:              "username",
		Password:              "password",
		PassCredentialsAll:    true,
		UserAgent:             "User-agent",
		Version:               "1.2.3",
	}
	assert.Equal(t, expected, opts)
}

type testPlugin struct {
	t   *testing.T
	dir string
}

func (t *testPlugin) Dir() string {
	return t.dir
}

func (t *testPlugin) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:       "fake-plugin",
		Type:       "cli/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config:     &schema.ConfigCLIV1{},
		RuntimeConfig: &plugin.RuntimeConfigSubprocess{
			PlatformCommand: []plugin.PlatformCommand{
				{
					Command: "echo fake-plugin",
				},
			},
		},
	}
}

func (t *testPlugin) Invoke(_ context.Context, _ *plugin.Input) (*plugin.Output, error) {
	// Simulate a plugin invocation
	output := &plugin.Output{
		Message: schema.OutputMessageGetterV1{
			Data: []byte("fake-plugin output"),
		},
	}
	return output, nil
}

var _ plugin.Plugin = (*testPlugin)(nil)

func TestGetterPlugin(t *testing.T) {
	gp := getterPlugin{
		options: []Option{},
		plg:     &testPlugin{t: t, dir: "fake/dir"},
	}

	buf, err := gp.Get("test://example.com", WithTimeout(5*time.Second))
	require.NoError(t, err)

	assert.Equal(t, "fake-plugin output", buf.String())
}

func TestCollectGetterPluginsPassesEnv(t *testing.T) {
	env := cli.New()
	env.PluginsDirectory = pluginDir
	env.Debug = true

	providers, err := collectGetterPlugins(env)
	require.NoError(t, err)
	require.NotEmpty(t, providers, "expected at least one plugin provider")

	getter, err := providers.ByScheme("test")
	require.NoError(t, err)

	gp, ok := getter.(*getterPlugin)
	require.True(t, ok, "expected getter to be a *getterPlugin")

	require.NotEmpty(t, gp.env, "expected env to be set on getterPlugin")
	envMap := plugin.ParseEnv(gp.env)

	assert.Contains(t, envMap, "HELM_DEBUG", "expected HELM_DEBUG in env")
	assert.Equal(t, "true", envMap["HELM_DEBUG"], "expected HELM_DEBUG to be true")
	assert.Contains(t, envMap, "HELM_PLUGINS", "expected HELM_PLUGINS in env")
	assert.Equal(t, pluginDir, envMap["HELM_PLUGINS"], "expected HELM_PLUGINS to match pluginsDirectory")
}
