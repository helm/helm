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
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

func TestSelectPlatformManifest(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	child := func(os, arch, variant string) ocispec.Descriptor {
		return ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    digest.FromString(os + "/" + arch + "/" + variant),
			Size:      1,
			Platform:  &ocispec.Platform{OS: os, Architecture: arch, Variant: variant},
		}
	}
	index := ocispec.Index{Manifests: []ocispec.Descriptor{
		child("linux", "amd64", ""),
		child("linux", "arm", "v6"),
		child("linux", "arm", "v7"),
		child("windows", "amd64", ""),
	}}
	data, err := json.Marshal(index)
	require.NoError(t, err)

	ociIndex := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, data)
	require.NoError(t, store.Push(ctx, ociIndex, bytes.NewReader(data)))
	plainManifest := child("linux", "amd64", "")

	tests := []struct {
		name      string
		root      ocispec.Descriptor
		platform  ocispec.Platform
		expected  ocispec.Descriptor
		expectErr string
	}{
		{
			name:     "selects matching os and architecture",
			root:     ociIndex,
			platform: ocispec.Platform{OS: "windows", Architecture: "amd64"},
			expected: child("windows", "amd64", ""),
		},
		{
			name:     "selects matching variant",
			root:     ociIndex,
			platform: ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			expected: child("linux", "arm", "v7"),
		},
		{
			name:     "empty variant selects first matching architecture",
			root:     ociIndex,
			platform: ocispec.Platform{OS: "linux", Architecture: "arm"},
			expected: child("linux", "arm", "v6"),
		},
		{
			name:      "no matching platform",
			root:      ociIndex,
			platform:  ocispec.Platform{OS: "darwin", Architecture: "arm64"},
			expectErr: "no manifest found for platform darwin/arm64",
		},
		{
			name:      "no matching variant",
			root:      ociIndex,
			platform:  ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v8"},
			expectErr: "no manifest found for platform linux/arm/v8",
		},
		{
			name:     "non-index root passes through unchanged",
			root:     plainManifest,
			platform: ocispec.Platform{OS: "darwin", Architecture: "arm64"},
			expected: plainManifest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectPlatformManifest(ctx, store, tt.root, &tt.platform)
			if tt.expectErr != "" {
				assert.ErrorContains(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
