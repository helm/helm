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

package v3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataImages(t *testing.T) {
	withImages := func(raw string) *Metadata {
		return &Metadata{Annotations: map[string]string{ImagesAnnotation: raw}}
	}

	tests := []struct {
		name   string
		md     *Metadata
		want   []*Image
		errMsg string
	}{
		{
			name: "nil metadata",
			md:   nil,
		},
		{
			name: "no annotations",
			md:   &Metadata{},
		},
		{
			name: "annotation not set",
			md:   &Metadata{Annotations: map[string]string{"category": "Infrastructure"}},
		},
		{
			name: "empty annotation",
			md:   withImages(""),
		},
		{
			name: "valid images",
			md: withImages(`- name: podinfo
  image: ghcr.io/stefanprodan/podinfo:6.0.0
- name: redis
  image: docker.io/library/redis:6.0.8
  dependency: redis
`),
			want: []*Image{
				{Name: "podinfo", Image: "ghcr.io/stefanprodan/podinfo:6.0.0"},
				{Name: "redis", Image: "docker.io/library/redis:6.0.8", Dependency: "redis"},
			},
		},
		{
			name: "unknown fields are ignored",
			md: withImages(`- name: redis
  image: docker.io/library/redis:6.0.8
  whitelisted: true
`),
			want: []*Image{
				{Name: "redis", Image: "docker.io/library/redis:6.0.8"},
			},
		},
		{
			name: "values are sanitized",
			md: withImages(`- name: "podinfo\u0081"
  image: "ghcr.io/stefanprodan/podinfo:6.0.0"
`),
			want: []*Image{
				{Name: "podinfo", Image: "ghcr.io/stefanprodan/podinfo:6.0.0"},
			},
		},
		{
			name: "annotation is a map, not a list",
			md: withImages(`name: podinfo
image: ghcr.io/stefanprodan/podinfo:6.0.0
`),
			errMsg: `annotation "helm.sh/images" is not a valid list of images`,
		},
		{
			name:   "annotation is invalid YAML",
			md:     withImages("- name: [podinfo"),
			errMsg: `annotation "helm.sh/images" is not a valid list of images`,
		},
		{
			name:   "null entry",
			md:     withImages("-\n"),
			errMsg: "images must not contain empty or null nodes",
		},
		{
			name:   "entry without name",
			md:     withImages("- image: ghcr.io/stefanprodan/podinfo:6.0.0\n"),
			errMsg: `image "ghcr.io/stefanprodan/podinfo:6.0.0" must have a name`,
		},
		{
			name:   "entry without image",
			md:     withImages("- name: podinfo\n"),
			errMsg: `image "podinfo" must have an image reference`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.md.Images()
			if tt.errMsg != "" {
				require.ErrorContains(t, err, tt.errMsg)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
