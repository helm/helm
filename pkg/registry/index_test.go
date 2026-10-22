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
		case path == "/v2/testrepo/multichart/blobs/"+imageIndexDigest.String(),
			path == "/v2/testrepo/multichart/manifests/"+imageIndexDigest.String():
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", imageIndexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imageIndexBytes)

		// Serve chart manifest by digest
		case path == "/v2/testrepo/multichart/manifests/"+chartManifestDigest.String():
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
	host := "localhost:" + u.Port()
	ref := host + "/testrepo/multichart:1.0.0"

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
		case path == "/v2/testrepo/nohelm/blobs/"+imageIndexDigest.String(),
			path == "/v2/testrepo/nohelm/manifests/"+imageIndexDigest.String():
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
	host := "localhost:" + u.Port()
	ref := host + "/testrepo/nohelm:1.0.0"

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
	host := "localhost:" + u.Port()
	ref := host + "/testrepo/singlechart:1.0.0"

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(ref)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "singlechart", result.Chart.Meta.Name)
}

// testChart is a chart manifest plus its config and layer blobs, for building
// multi-chart Image Index test fixtures.
type testChart struct {
	name, version  string
	configData     []byte
	layerData      []byte
	manifestBytes  []byte
	configDigest   digest.Digest
	layerDigest    digest.Digest
	manifestDigest digest.Digest
}

func newTestChart(name, version string) testChart {
	configData := []byte(`{"name":"` + name + `","version":"` + version + `","apiVersion":"v2"}`)
	layerData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	configDigest := computeDigest(configData)
	layerDigest := computeDigest(layerData)
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ConfigMediaType, Digest: configDigest, Size: int64(len(configData))},
		Layers:    []ocispec.Descriptor{{MediaType: ChartLayerMediaType, Digest: layerDigest, Size: int64(len(layerData))}},
	}
	manifestBytes, _ := json.Marshal(manifest)
	return testChart{
		name: name, version: version,
		configData: configData, layerData: layerData, manifestBytes: manifestBytes,
		configDigest: configDigest, layerDigest: layerDigest, manifestDigest: computeDigest(manifestBytes),
	}
}

func (c testChart) indexDescriptor() ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		Digest:       c.manifestDigest,
		Size:         int64(len(c.manifestBytes)),
		ArtifactType: ChartArtifactType,
		Annotations: map[string]string{
			ocispec.AnnotationTitle:   c.name,
			ocispec.AnnotationVersion: c.version,
		},
	}
}

// legacyIndexDescriptor is an index entry without artifactType or annotations,
// as produced before this feature; selection must fall back to the chart config.
func (c testChart) legacyIndexDescriptor() ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    c.manifestDigest,
		Size:      int64(len(c.manifestBytes)),
	}
}

// serveMultiChartIndex serves an Image Index (by tag 1.0.0 and by digest) plus
// every chart's manifest, config and layer (matched by digest), so a pull can
// resolve and select from it. Matching is repo-prefix agnostic.
func serveMultiChartIndex(indexBytes []byte, indexDigest digest.Digest, charts ...testChart) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/v2/":
			w.WriteHeader(http.StatusOK)
			return
		case strings.Contains(p, indexDigest.Encoded()),
			strings.Contains(p, "/manifests/") && !strings.Contains(p, "sha256:"):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", indexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
			return
		}
		for _, c := range charts {
			switch {
			case strings.Contains(p, c.manifestDigest.Encoded()):
				w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
				w.Header().Set("Docker-Content-Digest", c.manifestDigest.String())
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(c.manifestBytes)
				return
			case strings.Contains(p, c.configDigest.Encoded()):
				w.Header().Set("Content-Type", ConfigMediaType)
				w.Header().Set("Docker-Content-Digest", c.configDigest.String())
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(c.configData)
				return
			case strings.Contains(p, c.layerDigest.Encoded()):
				w.Header().Set("Content-Type", ChartLayerMediaType)
				w.Header().Set("Docker-Content-Digest", c.layerDigest.String())
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(c.layerData)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestPullFromImageIndexSelectsByName(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Order alpha, beta on purpose: selection must be by name, not position.
		Manifests: []ocispec.Descriptor{alpha.indexDescriptor(), beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := computeDigest(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// Pulling the "beta" reference must return beta even though alpha is first.
	result, err := client.Pull(host + "/testrepo/beta:1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "beta", result.Chart.Meta.Name)

	// And the "alpha" reference must return alpha from the same index.
	result, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "alpha", result.Chart.Meta.Name)
}

func TestPullFromImageIndexAmbiguousName(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{alpha.indexDescriptor(), beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := computeDigest(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// The "gamma" reference matches neither chart: the wrong chart must not be
	// returned silently; the error lists the available candidates.
	_, err = client.Pull(host + "/testrepo/gamma:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none is named")
	assert.Contains(t, err.Error(), "alpha")
	assert.Contains(t, err.Error(), "beta")
}

func TestPullFromImageIndexSelectsByVersion(t *testing.T) {
	v1 := newTestChart("app", "1.0.0")
	v2 := newTestChart("app", "2.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Same name, different versions: the requested version breaks the tie.
		Manifests: []ocispec.Descriptor{v1.indexDescriptor(), v2.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := computeDigest(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, v1, v2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assert.Equal(t, "app", result.Chart.Meta.Name)
	assert.Equal(t, "2.0.0", result.Chart.Meta.Version)
}

func TestPullFromImageIndexLegacySelectsByName(t *testing.T) {
	// Legacy index: entries carry neither artifactType nor title annotations, so
	// selection falls back to config.mediaType and the chart's own Chart.yaml.
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{alpha.legacyIndexDescriptor(), beta.legacyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := computeDigest(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/beta:1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "beta", result.Chart.Meta.Name)
}
