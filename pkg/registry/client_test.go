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

package registry

import (
	"io"
	"testing"

	"github.com/containerd/containerd/remotes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientResolverNotSupported(t *testing.T) {
	var r remotes.Resolver

	client, err := NewClient(ClientOptResolver(r))
	require.Equal(t, err, errDeprecatedRemote)
	assert.Nil(t, client)
}

func TestStripURL(t *testing.T) {
	client := &Client{
		out: io.Discard,
	}
	// no change with supported host formats
	assert.Equal(t, "username@localhost:8000", client.stripURL("username@localhost:8000"))
	assert.Equal(t, "localhost:8000", client.stripURL("localhost:8000"))
	assert.Equal(t, "docker.pkg.dev", client.stripURL("docker.pkg.dev"))

	// test strip scheme from host in URL
	assert.Equal(t, "docker.pkg.dev", client.stripURL("oci://docker.pkg.dev"))
	assert.Equal(t, "docker.pkg.dev", client.stripURL("http://docker.pkg.dev"))
	assert.Equal(t, "docker.pkg.dev", client.stripURL("https://docker.pkg.dev"))

	// test strip repo from Registry in URL
	assert.Equal(t, "127.0.0.1:15000", client.stripURL("127.0.0.1:15000/asdf"))
	assert.Equal(t, "127.0.0.1:15000", client.stripURL("127.0.0.1:15000/asdf/asdf"))
	assert.Equal(t, "127.0.0.1:15000", client.stripURL("127.0.0.1:15000"))
}
