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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
)

// TestPullPluginImageIndex verifies that a plugin published as a multi-platform
// image index resolves to the manifest for the current platform, and fails
// clearly when the index has no entry for it.
func TestPullPluginImageIndex(t *testing.T) {
	const repo = "testrepo/myplugin"
	blobs := map[string][]byte{}
	manifests := map[string][]byte{}
	mediaTypes := map[string]string{}

	push := func(store map[string][]byte, mediaType string, data []byte) ocispec.Descriptor {
		desc := content.NewDescriptorFromBytes(mediaType, data)
		store[desc.Digest.String()] = data
		mediaTypes[desc.Digest.String()] = mediaType
		return desc
	}
	marshal := func(v any) []byte {
		data, err := json.Marshal(v)
		require.NoError(t, err)
		return data
	}
	pluginManifest := func(platform ocispec.Platform) (ocispec.Descriptor, []byte) {
		pluginData := []byte("plugin for " + platform.OS + "/" + platform.Architecture)
		layer := push(blobs, "application/vnd.oci.image.layer.v1.tar+gzip", pluginData)
		layer.Annotations = map[string]string{ocispec.AnnotationTitle: "myplugin-1.0.0.tgz"}
		config := push(blobs, PluginArtifactType, []byte("{}"))
		desc := push(manifests, ocispec.MediaTypeImageManifest, marshal(ocispec.Manifest{
			Versioned: specs.Versioned{SchemaVersion: 2},
			MediaType: ocispec.MediaTypeImageManifest,
			Config:    config,
			Layers:    []ocispec.Descriptor{layer},
		}))
		desc.Platform = &platform
		return desc, pluginData
	}

	tagIndex := func(tag string, children ...ocispec.Descriptor) {
		desc := push(manifests, ocispec.MediaTypeImageIndex, marshal(ocispec.Index{
			Versioned: specs.Versioned{SchemaVersion: 2},
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: children,
		}))
		manifests[tag] = manifests[desc.Digest.String()]
		mediaTypes[tag] = ocispec.MediaTypeImageIndex
	}

	other, _ := pluginManifest(ocispec.Platform{OS: "plan9", Architecture: "mips"})
	native, nativeData := pluginManifest(ocispec.Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH})
	tagIndex("1.0.0", other, native)
	tagIndex("2.0.0", other)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var store map[string][]byte
		var ref string
		switch {
		case strings.HasPrefix(r.URL.Path, "/v2/"+repo+"/manifests/"):
			store, ref = manifests, strings.TrimPrefix(r.URL.Path, "/v2/"+repo+"/manifests/")
		case strings.HasPrefix(r.URL.Path, "/v2/"+repo+"/blobs/"):
			store, ref = blobs, strings.TrimPrefix(r.URL.Path, "/v2/"+repo+"/blobs/")
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data, ok := store[ref]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", mediaTypes[ref])
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Docker-Content-Digest", content.NewDescriptorFromBytes("", data).Digest.String())
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c, err := NewClient(
		ClientOptPlainHTTP(),
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
	)
	require.NoError(t, err)

	tests := []struct {
		name      string
		tag       string
		expectErr string
	}{
		{
			name: "resolves manifest for current platform",
			tag:  "1.0.0",
		},
		{
			name:      "no manifest for current platform",
			tag:       "2.0.0",
			expectErr: "no manifest found for platform " + runtime.GOOS + "/" + runtime.GOARCH,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := strings.TrimPrefix(srv.URL, "http://") + "/" + repo + ":" + tt.tag
			result, err := c.PullPlugin(ref, "myplugin")
			if tt.expectErr != "" {
				assert.ErrorContains(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, native.Digest, result.Manifest.Digest)
			assert.Equal(t, nativeData, result.PluginData)
		})
	}
}

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
