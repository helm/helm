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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// computeDigest calculates the SHA256 digest of data.
func computeDigest(data []byte) digest.Digest {
	h := sha256.Sum256(data)
	return digest.NewDigestFromBytes(digest.SHA256, h[:])
}

func TestPullFromImageIndex(t *testing.T) {
	// Build chart config and layer with real digests
	chartConfigData := []byte(`{"name":"testchart","version":"1.0.0","apiVersion":"v2"}`)
	chartConfigDigest := computeDigest(chartConfigData)

	// Minimal valid gzipped content
	chartLayerData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	chartLayerDigest := computeDigest(chartLayerData)

	// Create chart manifest with real digests
	chartManifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: ConfigMediaType,
			Digest:    chartConfigDigest,
			Size:      int64(len(chartConfigData)),
		},
		Layers: []ocispec.Descriptor{
			{
				MediaType: ChartLayerMediaType,
				Digest:    chartLayerDigest,
				Size:      int64(len(chartLayerData)),
			},
		},
	}
	chartManifestBytes, _ := json.Marshal(chartManifest)
	chartManifestDigest := computeDigest(chartManifestBytes)

	// Container manifest (we won't actually serve the blobs, just need valid structure)
	containerManifestDigest := digest.Digest("sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	// Image Index containing both chart and container manifests
	imageIndex := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Digest:       containerManifestDigest,
				Size:         500,
				ArtifactType: "application/vnd.oci.image.config.v1+json",
				Platform: &ocispec.Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Digest:       chartManifestDigest,
				Size:         int64(len(chartManifestBytes)),
				ArtifactType: ChartArtifactType,
			},
		},
	}
	imageIndexBytes, _ := json.Marshal(imageIndex)
	imageIndexDigest := computeDigest(imageIndexBytes)

	// Create test server that serves the Image Index
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/v2/":
			w.WriteHeader(http.StatusOK)

		case path == "/v2/testrepo/multichart/manifests/1.0.0":
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", imageIndexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imageIndexBytes)

		// Serve Image Index by digest (for resolveFromIndex FetchAll)
		case path == fmt.Sprintf("/v2/testrepo/multichart/blobs/%s", imageIndexDigest.String()),
			path == fmt.Sprintf("/v2/testrepo/multichart/manifests/%s", imageIndexDigest.String()):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", imageIndexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imageIndexBytes)

		// Serve chart manifest by digest
		case path == fmt.Sprintf("/v2/testrepo/multichart/manifests/%s", chartManifestDigest.String()):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", chartManifestDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(chartManifestBytes)

		// Serve chart config blob
		case strings.Contains(path, chartConfigDigest.Encoded()):
			w.Header().Set("Content-Type", ConfigMediaType)
			w.Header().Set("Docker-Content-Digest", chartConfigDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(chartConfigData)

		// Serve chart layer blob
		case strings.Contains(path, chartLayerDigest.Encoded()):
			w.Header().Set("Content-Type", ChartLayerMediaType)
			w.Header().Set("Docker-Content-Digest", chartLayerDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(chartLayerData)

		default:
			t.Logf("404 for path: %s", path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	u, _ := url.Parse(s.URL)
	host := fmt.Sprintf("localhost:%s", u.Port())
	ref := fmt.Sprintf("%s/testrepo/multichart:1.0.0", host)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// Pull should automatically select the chart manifest from the index
	result, err := client.Pull(ref)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "testchart", result.Chart.Meta.Name)
	assert.Equal(t, "1.0.0", result.Chart.Meta.Version)
}

func TestPullFromImageIndexNoMatchingArtifactType(t *testing.T) {
	// Image Index with only container images, no Helm chart
	imageIndex := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Digest:       "sha256:2222222222222222222222222222222222222222222222222222222222222222",
				Size:         500,
				ArtifactType: "application/vnd.oci.image.config.v1+json",
				Platform: &ocispec.Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Digest:       "sha256:3333333333333333333333333333333333333333333333333333333333333333",
				Size:         500,
				ArtifactType: "application/vnd.oci.image.config.v1+json",
				Platform: &ocispec.Platform{
					Architecture: "arm64",
					OS:           "linux",
				},
			},
		},
	}
	imageIndexBytes, _ := json.Marshal(imageIndex)
	imageIndexDigest := computeDigest(imageIndexBytes)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case path == "/v2/testrepo/nohelm/manifests/1.0.0":
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", imageIndexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imageIndexBytes)
		// Serve Image Index by digest
		case path == fmt.Sprintf("/v2/testrepo/nohelm/blobs/%s", imageIndexDigest.String()),
			path == fmt.Sprintf("/v2/testrepo/nohelm/manifests/%s", imageIndexDigest.String()):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", imageIndexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imageIndexBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	u, _ := url.Parse(s.URL)
	host := fmt.Sprintf("localhost:%s", u.Port())
	ref := fmt.Sprintf("%s/testrepo/nohelm:1.0.0", host)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest with artifactType")
	assert.Contains(t, err.Error(), ChartArtifactType)
}

func TestPullSingleManifestNotIndex(t *testing.T) {
	// Regular manifest (not an Index) should work as before
	// Build config and layer with real digests
	configData := []byte(`{"name":"singlechart","version":"1.0.0","apiVersion":"v2"}`)
	configDigest := computeDigest(configData)

	layerData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	layerDigest := computeDigest(layerData)

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: ConfigMediaType,
			Digest:    configDigest,
			Size:      int64(len(configData)),
		},
		Layers: []ocispec.Descriptor{
			{
				MediaType: ChartLayerMediaType,
				Digest:    layerDigest,
				Size:      int64(len(layerData)),
			},
		},
	}
	manifestBytes, _ := json.Marshal(manifest)
	manifestDigest := computeDigest(manifestBytes)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/v2/":
			w.WriteHeader(http.StatusOK)

		case path == "/v2/testrepo/singlechart/manifests/1.0.0":
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", manifestDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(manifestBytes)

		case strings.Contains(path, configDigest.Encoded()):
			w.Header().Set("Content-Type", ConfigMediaType)
			w.Header().Set("Docker-Content-Digest", configDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(configData)

		case strings.Contains(path, layerDigest.Encoded()):
			w.Header().Set("Content-Type", ChartLayerMediaType)
			w.Header().Set("Docker-Content-Digest", layerDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerData)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	u, _ := url.Parse(s.URL)
	host := fmt.Sprintf("localhost:%s", u.Port())
	ref := fmt.Sprintf("%s/testrepo/singlechart:1.0.0", host)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(ref)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "singlechart", result.Chart.Meta.Name)
}
