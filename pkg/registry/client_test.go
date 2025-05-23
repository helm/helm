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
	"context"
	"io"
	"testing"

	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
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

// Inspired by oras test
// https://github.com/oras-project/oras-go/blob/05a2b09cbf2eab1df691411884dc4df741ec56ab/content_test.go#L1802
func TestTagManifestTransformsReferences(t *testing.T) {
	memStore := memory.New()
	client := &Client{out: io.Discard}
	ctx := context.Background()

	refWithPlus := "test-registry.io/charts/test:1.0.0+metadata"
	expectedRef := "test-registry.io/charts/test:1.0.0_metadata" // + becomes _

	configDesc := ocispec.Descriptor{MediaType: ConfigMediaType, Digest: "sha256:config", Size: 100}
	layers := []ocispec.Descriptor{{MediaType: ChartLayerMediaType, Digest: "sha256:layer", Size: 200}}

	parsedRef, err := newReference(refWithPlus)
	require.NoError(t, err)

	desc, err := client.tagManifest(ctx, memStore, configDesc, layers, nil, parsedRef)
	require.NoError(t, err)

	transformedDesc, err := memStore.Resolve(ctx, expectedRef)
	require.NoError(t, err, "Should find the reference with _ instead of +")
	require.Equal(t, desc.Digest, transformedDesc.Digest)

	_, err = memStore.Resolve(ctx, refWithPlus)
	require.Error(t, err, "Should NOT find the reference with the original +")
}
