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

package pusher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/registry"
)

func TestProvider(t *testing.T) {
	p := Provider{
		[]string{"one", "three"},
		func(_ ...Option) (Pusher, error) { return nil, nil },
	}

	assert.True(t, p.Provides("three"), "Expected provider to provide three")
}

func TestProviders(t *testing.T) {
	ps := Providers{
		{[]string{"one", "three"}, func(_ ...Option) (Pusher, error) { return nil, nil }},
		{[]string{"two", "four"}, func(_ ...Option) (Pusher, error) { return nil, nil }},
	}

	_, err := ps.ByScheme("one")
	require.NoError(t, err)
	_, err = ps.ByScheme("four")
	require.NoError(t, err)

	_, err = ps.ByScheme("five")
	assert.Error(t, err, "Did not expect handler for five")
}

func TestAll(t *testing.T) {
	env := cli.New()
	all := All(env)
	assert.Len(t, all, 1, "expected 1 provider (OCI), got %d", len(all))
}

func TestByScheme(t *testing.T) {
	env := cli.New()
	g := All(env)
	_, err := g.ByScheme(registry.OCIScheme)
	assert.NoError(t, err)
}
