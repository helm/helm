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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
)

// Inspired by oras test
// https://github.com/oras-project/oras-go/blob/05a2b09cbf2eab1df691411884dc4df741ec56ab/content_test.go#L1802
func TestTagManifestTransformsReferences(t *testing.T) {
	memStore := memory.New()
	client := &Client{out: io.Discard}
	ctx := t.Context()

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
