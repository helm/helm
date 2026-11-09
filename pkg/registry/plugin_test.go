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
	"context"
	"encoding/json"
	"io"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, store.Push(context.Background(), desc, bytes.NewReader(data)))
	return desc
}

func newImageIndex(manifests ...ocispec.Descriptor) ocispec.Index {
	return ocispec.Index{
		SchemaVersion: 2,
		MediaType:     ocispec.MediaTypeImageIndex,
		Manifests:     manifests,
	}
}

// newPluginStore returns a store holding a plugin manifest for "demo" with
// pluginData as its tarball, and a manifest of an unrelated artifact type
func newPluginStore(t *testing.T, pluginData []byte) (*memory.Store, ocispec.Descriptor, ocispec.Descriptor) {
	t.Helper()
	store := memory.New()
	layer := pushBlob(t, store, "application/vnd.oci.image.layer.v1.tar+gzip", pluginData)
	layer.Annotations = map[string]string{ocispec.AnnotationTitle: "demo-1.0.0.tgz"}
	config := pushBlob(t, store, ocispec.MediaTypeEmptyJSON, []byte("{}"))
	pluginManifest := pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		SchemaVersion: 2,
		MediaType:     ocispec.MediaTypeImageManifest,
		ArtifactType:  PluginArtifactType,
		Config:        config,
		Layers:        []ocispec.Descriptor{layer},
	})
	otherManifest := pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		SchemaVersion: 2,
		MediaType:     ocispec.MediaTypeImageManifest,
		ArtifactType:  "application/vnd.example.other",
		Config:        config,
		Layers:        []ocispec.Descriptor{},
	})
	return store, pluginManifest, otherManifest
}

func TestProcessPluginPull(t *testing.T) {
	pluginData := []byte("plugin tarball")

	client, err := NewClient(ClientOptWriter(io.Discard))
	require.NoError(t, err)

	t.Run("manifest", func(t *testing.T) {
		store, pluginManifest, _ := newPluginStore(t, pluginData)
		result, err := client.processPluginPull(&GenericPullResult{
			Manifest:    pluginManifest,
			MemoryStore: store,
			Ref:         "localhost:5000/plugins/demo:1.0.0",
		}, "demo")
		require.NoError(t, err)
		assert.Equal(t, pluginData, result.PluginData)
	})

	t.Run("image index", func(t *testing.T) {
		store, pluginManifest, otherManifest := newPluginStore(t, pluginData)
		index := pushJSON(t, store, ocispec.MediaTypeImageIndex, newImageIndex(otherManifest, pluginManifest))
		result, err := client.processPluginPull(&GenericPullResult{
			Manifest:    index,
			MemoryStore: store,
			Ref:         "localhost:5000/plugins/demo:idx",
		}, "demo")
		require.NoError(t, err)
		assert.Equal(t, pluginData, result.PluginData)
		assert.Equal(t, index.Digest, result.Manifest.Digest)
	})

	t.Run("image index without a plugin manifest", func(t *testing.T) {
		store, _, otherManifest := newPluginStore(t, pluginData)
		index := pushJSON(t, store, ocispec.MediaTypeImageIndex, newImageIndex(otherManifest))
		_, err := client.processPluginPull(&GenericPullResult{
			Manifest:    index,
			MemoryStore: store,
			Ref:         "localhost:5000/plugins/demo:idx",
		}, "demo")
		require.ErrorContains(t, err, "image index does not contain a manifest with artifact type "+PluginArtifactType)
	})
}
