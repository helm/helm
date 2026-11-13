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
	"bytes"
	"encoding/json"
	"io"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

func TestGetPluginName(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		expected  string
		expectErr bool
	}{
		{
			name:     "valid OCI reference with tag",
			source:   "oci://ghcr.io/user/plugin-name:v1.0.0",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference with digest",
			source:   "oci://ghcr.io/user/plugin-name@sha256:1234567890abcdef",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference without tag",
			source:   "oci://ghcr.io/user/plugin-name",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference with multiple path segments",
			source:   "oci://registry.example.com/org/team/plugin-name:latest",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference with plus signs in tag",
			source:   "oci://registry.example.com/user/plugin-name:v1.0.0+build.1",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference - single path segment",
			source:   "oci://registry.example.com/plugin",
			expected: "plugin",
		},
		{
			name:      "invalid OCI reference - no repository",
			source:    "oci://registry.example.com",
			expectErr: true,
		},
		{
			name:      "invalid OCI reference - malformed",
			source:    "not-an-oci-reference",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginName, err := GetPluginName(tt.source)

			if tt.expectErr {
				assert.Error(t, err, "expected error but got none")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, pluginName)
			}
		})
	}
}

func pushJSON(t *testing.T, store *memory.Store, mediaType string, v any) ocispec.Descriptor {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return pushBlob(t, store, mediaType, data)
}

func pushBlob(t *testing.T, store *memory.Store, mediaType string, data []byte) ocispec.Descriptor {
	t.Helper()
	desc := content.NewDescriptorFromBytes(mediaType, data)
	require.NoError(t, store.Push(t.Context(), desc, bytes.NewReader(data)))
	return desc
}

func newImageIndex(manifests ...ocispec.Descriptor) ocispec.Index {
	return ocispec.Index{
		SchemaVersion: 2,
		MediaType:     ocispec.MediaTypeImageIndex,
		Manifests:     manifests,
	}
}

// pluginFixture is a store holding a plugin manifest for "demo" and the
// manifest of an unrelated artifact type, each with one tar+gzip layer
type pluginFixture struct {
	store          *memory.Store
	pluginManifest ocispec.Descriptor
	pluginLayer    ocispec.Descriptor
	otherManifest  ocispec.Descriptor
	otherLayer     ocispec.Descriptor
}

func newPluginFixture(t *testing.T, pluginData []byte) pluginFixture {
	t.Helper()
	f := pluginFixture{store: memory.New()}
	f.pluginLayer = pushBlob(t, f.store, "application/vnd.oci.image.layer.v1.tar+gzip", pluginData)
	f.pluginLayer.Annotations = map[string]string{ocispec.AnnotationTitle: "demo-1.0.0.tgz"}
	f.otherLayer = pushBlob(t, f.store, "application/vnd.oci.image.layer.v1.tar+gzip", []byte("other platform layer"))
	config := pushBlob(t, f.store, ocispec.MediaTypeEmptyJSON, []byte("{}"))
	f.pluginManifest = pushJSON(t, f.store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		SchemaVersion: 2,
		MediaType:     ocispec.MediaTypeImageManifest,
		ArtifactType:  PluginArtifactType,
		Config:        config,
		Layers:        []ocispec.Descriptor{f.pluginLayer},
	})
	f.otherManifest = pushJSON(t, f.store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		SchemaVersion: 2,
		MediaType:     ocispec.MediaTypeImageManifest,
		ArtifactType:  "application/vnd.example.other",
		Config:        config,
		Layers:        []ocispec.Descriptor{f.otherLayer},
	})
	return f
}

func TestProcessPluginPull(t *testing.T) {
	pluginData := []byte("plugin tarball")

	client, err := NewClient(ClientOptWriter(io.Discard))
	require.NoError(t, err)

	t.Run("manifest", func(t *testing.T) {
		f := newPluginFixture(t, pluginData)
		result, err := client.processPluginPull(&GenericPullResult{
			Manifest:    f.pluginManifest,
			MemoryStore: f.store,
			Ref:         "localhost:5000/plugins/demo:1.0.0",
		}, f.pluginManifest, "demo")
		require.NoError(t, err)
		assert.Equal(t, pluginData, result.PluginData)
		assert.Equal(t, f.pluginManifest.Digest, result.Manifest.Digest)
	})

	t.Run("manifest selected from an image index", func(t *testing.T) {
		f := newPluginFixture(t, pluginData)
		index := pushJSON(t, f.store, ocispec.MediaTypeImageIndex, newImageIndex(f.otherManifest, f.pluginManifest))
		result, err := client.processPluginPull(&GenericPullResult{
			Manifest:    f.pluginManifest,
			MemoryStore: f.store,
			Ref:         "localhost:5000/plugins/demo:idx",
		}, index, "demo")
		require.NoError(t, err)
		assert.Equal(t, pluginData, result.PluginData)
		assert.Equal(t, index.Digest, result.Manifest.Digest)
	})
}

func TestPluginManifestFromIndex(t *testing.T) {
	t.Run("plugin manifest", func(t *testing.T) {
		f := newPluginFixture(t, []byte("plugin tarball"))
		index := pushJSON(t, f.store, ocispec.MediaTypeImageIndex, newImageIndex(f.otherManifest, f.pluginManifest))
		desc, err := pluginManifestFromIndex(t.Context(), f.store, index)
		require.NoError(t, err)
		assert.Equal(t, f.pluginManifest.Digest, desc.Digest)
	})

	t.Run("no plugin manifest", func(t *testing.T) {
		f := newPluginFixture(t, []byte("plugin tarball"))
		index := pushJSON(t, f.store, ocispec.MediaTypeImageIndex, newImageIndex(f.otherManifest))
		_, err := pluginManifestFromIndex(t.Context(), f.store, index)
		require.ErrorContains(t, err, "image index does not contain a manifest with artifact type "+PluginArtifactType)
	})
}

func TestPluginPullOptionsCopiesOnlyThePluginManifest(t *testing.T) {
	ctx := t.Context()
	f := newPluginFixture(t, []byte("plugin tarball"))
	index := pushJSON(t, f.store, ocispec.MediaTypeImageIndex, newImageIndex(f.otherManifest, f.pluginManifest))
	require.NoError(t, f.store.Tag(ctx, index, "idx"))

	var root ocispec.Descriptor
	opts := pluginPullOptions(&root)
	dst := memory.New()
	copied, err := oras.Copy(ctx, f.store, "idx", dst, "", oras.CopyOptions{MapRoot: opts.MapRoot})
	require.NoError(t, err)

	assert.Equal(t, index.Digest, root.Digest)
	assert.Equal(t, f.pluginManifest.Digest, copied.Digest)
	for _, desc := range []ocispec.Descriptor{f.pluginManifest, f.pluginLayer} {
		ok, err := dst.Exists(ctx, desc)
		require.NoError(t, err)
		assert.True(t, ok, "expected %s to be copied", desc.Digest)
	}
	for _, desc := range []ocispec.Descriptor{index, f.otherManifest, f.otherLayer} {
		ok, err := dst.Exists(ctx, desc)
		require.NoError(t, err)
		assert.False(t, ok, "expected %s not to be copied", desc.Digest)
	}
}
