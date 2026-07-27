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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
)

const pluginDir = "testdata/plugins"

func TestProvider(t *testing.T) {
	p := Provider{
		[]string{"one", "three"},
		func(_ ...Option) (Getter, error) { return nil, nil },
	}

	assert.True(t, p.Provides("three"), "Expected provider to provide three")
}

func TestProviders(t *testing.T) {
	ps := Providers{
		{[]string{"one", "three"}, func(_ ...Option) (Getter, error) { return nil, nil }},
		{[]string{"two", "four"}, func(_ ...Option) (Getter, error) { return nil, nil }},
	}

	_, err := ps.ByScheme("one")
	require.NoError(t, err)
	_, err = ps.ByScheme("four")
	require.NoError(t, err)

	_, err = ps.ByScheme("five")
	assert.Error(t, err, "Did not expect handler for five")
}

func TestProvidersWithTimeout(t *testing.T) {
	want := time.Hour
	getters := Getters(WithTimeout(want))
	getter, err := getters.ByScheme("http")
	require.NoError(t, err)
	httpGetter := getter.(*HTTPGetter)
	client, err := httpGetter.httpClient(httpGetter.opts)
	require.NoError(t, err)
	got := client.Timeout
	assert.Equal(t, want, got, "Expected %q, got %q", want, got)
}

func TestAll(t *testing.T) {
	env := cli.New()
	env.PluginsDirectory = pluginDir

	all := All(env)
	assert.Len(t, all, 4, "expected 4 providers (default plus three plugins), got %d", len(all))

	_, err := all.ByScheme("test2")
	assert.NoError(t, err)
}

func TestByScheme(t *testing.T) {
	env := cli.New()
	env.PluginsDirectory = pluginDir

	g := All(env)
	_, err := g.ByScheme("test")
	require.NoError(t, err)
	_, err = g.ByScheme("https")
	assert.NoError(t, err)
}
