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
	"compress/gzip"
	"encoding/json"
	"errors"
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

// gzipBytes returns payload as a gzip stream, standing in for a chart archive.
func gzipBytes(payload string) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(payload))
	_ = zw.Close()
	return buf.Bytes()
}

func TestPullFromImageIndex(t *testing.T) {
	// Build chart config and layer with real digests
	chartConfigData := []byte(`{"name":"testchart","version":"1.0.0","apiVersion":"v2"}`)
	chartConfigDigest := digest.FromBytes(chartConfigData)

	// Minimal valid gzipped content
	chartLayerData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	chartLayerDigest := digest.FromBytes(chartLayerData)

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
	chartManifestDigest := digest.FromBytes(chartManifestBytes)

	// The container manifest beside the chart, served like the registry serves it,
	// since selection reads the config of an entry whose declaration did not match.
	containerManifestBytes, _ := json.Marshal(ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.FromString("container-config"),
			Size:      10,
		},
	})
	containerManifestDigest := digest.FromBytes(containerManifestBytes)

	// Image Index containing both chart and container manifests
	imageIndex := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Digest:       containerManifestDigest,
				Size:         int64(len(containerManifestBytes)),
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
	imageIndexDigest := digest.FromBytes(imageIndexBytes)

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

		// Serve the container manifest by digest
		case path == "/v2/testrepo/multichart/manifests/"+containerManifestDigest.String():
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", containerManifestDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(containerManifestBytes)

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
	require.NotNil(t, result)
	assert.Equal(t, chartManifestDigest.String(), result.Manifest.Digest)
	assert.Equal(t, "testchart", result.Chart.Meta.Name)
	assert.Equal(t, "1.0.0", result.Chart.Meta.Version)
	assert.Equal(t, chartLayerData, result.Chart.Data)
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
	imageIndexDigest := digest.FromBytes(imageIndexBytes)

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
	// The types that were there have to be named, otherwise the message tells the
	// user what is missing without telling them what the index actually holds.
	assert.Contains(t, err.Error(), "application/vnd.oci.image.config.v1+json")
}

func TestPullSingleManifestNotIndex(t *testing.T) {
	// Regular manifest (not an Index) should work as before
	// Build config and layer with real digests
	configData := []byte(`{"name":"singlechart","version":"1.0.0","apiVersion":"v2"}`)
	configDigest := digest.FromBytes(configData)

	layerData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	layerDigest := digest.FromBytes(layerData)

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
	manifestDigest := digest.FromBytes(manifestBytes)

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

// newTestChart builds a chart fixture. The optional seed distinguishes two charts
// that share a name and version, the way two pushes of one chart version differ by
// their creation annotation.
func newTestChart(name, version string, seed ...string) testChart {
	configData := []byte(`{"name":"` + name + `","version":"` + version + `","apiVersion":"v2"}`)
	// Distinct payload per chart, so tests can tell the charts apart by layer.
	layerData := gzipBytes(name + "-" + version + strings.Join(seed, ""))
	configDigest := digest.FromBytes(configData)
	layerDigest := digest.FromBytes(layerData)
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ConfigMediaType, Digest: configDigest, Size: int64(len(configData))},
		Layers:    []ocispec.Descriptor{{MediaType: ChartLayerMediaType, Digest: layerDigest, Size: int64(len(layerData))}},
	}
	manifestBytes, _ := json.Marshal(manifest)
	return testChart{
		name: name, version: version,
		configData: configData, layerData: layerData, manifestBytes: manifestBytes,
		configDigest: configDigest, layerDigest: layerDigest, manifestDigest: digest.FromBytes(manifestBytes),
	}
}

// assertSelected checks that a pull returned the requested chart: its manifest,
// its metadata and its archive. All three are needed. The manifest digest pins
// that a manifest was selected at all, since a pull that copies the index graph
// reports the index digest here; metadata and archive are checked separately
// because they are read from different descriptors and can disagree.
func assertSelected(t *testing.T, want testChart, got *PullResult) {
	t.Helper()
	assert.Equal(t, want.manifestDigest.String(), got.Manifest.Digest)
	assert.Equal(t, want.name, got.Chart.Meta.Name)
	assert.Equal(t, want.version, got.Chart.Meta.Version)
	assert.Equal(t, want.layerData, got.Chart.Data)
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

// platformStampedIndexDescriptor declares the chart artifact type and also carries
// a platform, the way tooling that stamps one on every index entry leaves it.
func (c testChart) platformStampedIndexDescriptor() ocispec.Descriptor {
	desc := c.indexDescriptor()
	desc.Platform = &ocispec.Platform{OS: "unknown", Architecture: "unknown"}
	return desc
}

// declaredWithoutAnnotations announces the artifact type but no identity, so a
// selector has to read the chart's own config to learn its name and version.
func (c testChart) declaredWithoutAnnotations() ocispec.Descriptor {
	desc := c.indexDescriptor()
	desc.Annotations = nil
	return desc
}

// titleOnlyIndexDescriptor names the chart but omits the version annotation,
// which is what tooling that copies only the title leaves behind.
func (c testChart) titleOnlyIndexDescriptor() ocispec.Descriptor {
	desc := c.indexDescriptor()
	delete(desc.Annotations, ocispec.AnnotationVersion)
	return desc
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

type rawBlob struct {
	mediaType string
	data      []byte
}

// serveAlso answers for extra blobs by digest and delegates the rest to base. Index
// trees need it: everything a nested index lists has to be fetchable, or the pass
// over it is incomplete for a reason the fixture invented.
func serveAlso(base *httptest.Server, extras map[digest.Digest]rawBlob) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for d, blob := range extras {
			if strings.Contains(r.URL.Path, d.Encoded()) {
				w.Header().Set("Content-Type", blob.mediaType)
				w.Header().Set("Docker-Content-Digest", d.String())
				_, _ = w.Write(blob.data)
				return
			}
		}
		base.Config.Handler.ServeHTTP(w, r)
	}))
}

// indexHolding builds an index over the given entries and returns it as a blob.
func indexHolding(entries ...ocispec.Descriptor) (ocispec.Descriptor, rawBlob) {
	body, _ := json.Marshal(ocispec.Index{MediaType: ocispec.MediaTypeImageIndex, Manifests: entries})
	d := digest.FromBytes(body)
	return ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageIndex,
			Digest:    d,
			Size:      int64(len(body)),
		}, rawBlob{
			mediaType: ocispec.MediaTypeImageIndex,
			data:      body,
		}
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
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// Pulling the "beta" reference must return beta even though alpha is first.
	result, err := client.Pull(host + "/testrepo/beta:1.0.0")
	require.NoError(t, err)
	assertSelected(t, beta, result)

	// And the "alpha" reference must return alpha from the same index.
	result, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexNoChartWithRequestedName(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{alpha.indexDescriptor(), beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

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
	// Selection runs inside the copy, so every message it produces reaches the caller
	// wrapped in the copy's own error unless that wrapper is undone.
	assert.NotContains(t, err.Error(), "MapRoot")
}

func TestPullFromImageIndexSelectsPlatformStampedChart(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// The entry declares the chart artifact type and a platform at once. The
		// declared type decides; the platform must not hide the chart.
		Manifests: []ocispec.Descriptor{alpha.platformStampedIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexRequestedVersionNotInIndex(t *testing.T) {
	v1 := newTestChart("app", "1.0.0")
	v2 := newTestChart("app", "2.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{v1.indexDescriptor(), v2.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, v1, v2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// The name matches two charts and the requested version matches neither, so
	// the error has to say the version is missing, not ask for one.
	_, err = client.Pull(host + "/testrepo/app:3.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `none is version "3.0.0"`)
	assert.Contains(t, err.Error(), "app:1.0.0")
	assert.Contains(t, err.Error(), "app:2.0.0")
}

func TestPullFromImageIndexLegacyPlatformStampedChart(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	desc := alpha.legacyIndexDescriptor()
	desc.Platform = &ocispec.Platform{OS: "unknown", Architecture: "unknown"}
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Neither an artifact type nor annotations, and a platform on top. The
		// config mediaType is the only thing left that can identify the chart.
		Manifests: []ocispec.Descriptor{desc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexMixedArtifactTypeSelectsRequestedChart(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Mixed index: beta declares the artifact type, alpha predates it. Matching
		// on artifactType alone leaves exactly one candidate, and returning that one
		// unchecked would answer a request for alpha with beta.
		Manifests: []ocispec.Descriptor{beta.indexDescriptor(), alpha.legacyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)

	// The declared side of the same index still resolves.
	result, err = client.Pull(host + "/testrepo/beta:1.0.0")
	require.NoError(t, err)
	assertSelected(t, beta, result)
}

func TestPullFromImageIndexMixedArtifactTypeSelectsRequestedVersion(t *testing.T) {
	v1 := newTestChart("app", "1.0.0")
	v2 := newTestChart("app", "2.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Same name on both sides of the mixed index, so only the version tells them
		// apart, and the declared one is not the version being asked for.
		Manifests: []ocispec.Descriptor{v1.indexDescriptor(), v2.legacyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, v1, v2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assertSelected(t, v2, result)

	result, err = client.Pull(host + "/testrepo/app:1.0.0")
	require.NoError(t, err)
	assertSelected(t, v1, result)
}

func TestPullFromImageIndexSplitIdentityDoesNotSuppressFallback(t *testing.T) {
	appV1 := newTestChart("app", "1.0.0")
	otherV2 := newTestChart("other", "2.0.0")
	appV2 := newTestChart("app", "2.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Between them the declared entries announce the requested name and the
		// requested version, but neither announces both. The chart that does is the
		// undeclared one, so checking the two attributes separately would skip the
		// fallback pass and answer with app 1.0.0.
		Manifests: []ocispec.Descriptor{
			appV1.indexDescriptor(),
			otherV2.indexDescriptor(),
			appV2.legacyIndexDescriptor(),
		},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, appV1, otherV2, appV2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assertSelected(t, appV2, result)
}

func TestPullFromImageIndexUnreadableEntryDoesNotYieldAnotherChart(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{beta.indexDescriptor(), alpha.legacyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	// beta is readable and declares the artifact type; alpha, the chart actually
	// requested, cannot be fetched. One survivor must not become the answer.
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/v2/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(p, alpha.manifestDigest.Encoded()):
			// 404 rather than 500: both land in the skipped list, and this one does
			// not spend the test on the client's retry backoff.
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(p, indexDigest.Encoded()),
			strings.Contains(p, "/manifests/") && !strings.Contains(p, "sha256:"):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", indexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
		case strings.Contains(p, beta.manifestDigest.Encoded()):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", beta.manifestDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(beta.manifestBytes)
		case strings.Contains(p, beta.configDigest.Encoded()):
			w.Header().Set("Content-Type", ConfigMediaType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(beta.configData)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not be read")
	assert.NotContains(t, err.Error(), "no manifest with artifactType")
}

func TestPullFromImageIndexUnreadableEntryDoesNotYieldAnotherVersion(t *testing.T) {
	v1 := newTestChart("app", "1.0.0")
	v2 := newTestChart("app", "2.0.0")
	other := newTestChart("other", "9.9.9")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Two readable charts keep the count above one, so selection goes through the
		// name filter; the version actually requested lives in the entry that cannot
		// be read. The survivor named "app" must not be handed back for it.
		Manifests: []ocispec.Descriptor{
			v1.indexDescriptor(), other.indexDescriptor(), v2.legacyIndexDescriptor(),
		},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest, v1, other, v2)
	defer base.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, v2.manifestDigest.Encoded()) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		base.Config.Handler.ServeHTTP(w, r)
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/app:2.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not be read")
	// Naming both versions is the point: the reader has to see that 1.0.0 was found
	// and 2.0.0 was asked for, rather than receive 1.0.0 as though it were the answer.
	assert.Contains(t, err.Error(), `is version "1.0.0", not "2.0.0"`)
}

func TestPullFromImageIndexUnreadableIdentityIsNotAnAbsentChart(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Both declare the artifact type, so both are candidates without any fallback.
		// alpha carries no annotations, so learning that it is alpha needs a fetch,
		// and that fetch is the one the registry refuses.
		Manifests: []ocispec.Descriptor{alpha.declaredWithoutAnnotations(), beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer base.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, alpha.manifestDigest.Encoded()) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		base.Config.Handler.ServeHTTP(w, r)
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// "no chart named alpha here" would be a claim about the index. What actually
	// happened is that alpha's identity could not be read, and the message has to
	// say so rather than report an absence it never established.
	_, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not be read")
}

func TestPullFromImageIndexOneNameMatchIgnoresRequestedVersion(t *testing.T) {
	app := newTestChart("app", "1.0.0")
	other := newTestChart("other", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{app.indexDescriptor(), other.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, app, other)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// Only one chart is named "app", so there is no choice for the version to make
	// and it is not used to reject the answer. This matches a reference to a plain
	// manifest, where the tag decides and the chart's own version is not checked.
	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assertSelected(t, app, result)
}

func TestPullFromImageIndexUnreadableSoleIdentityIsNotDescribed(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// beta declares no type, so the fallback runs and fails on it, leaving alpha
		// as the sole candidate — and alpha's own identity cannot be read either.
		Manifests: []ocispec.Descriptor{alpha.declaredWithoutAnnotations(), beta.legacyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer base.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.Contains(p, alpha.manifestDigest.Encoded()) || strings.Contains(p, beta.manifestDigest.Encoded()) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		base.Config.Handler.ServeHTTP(w, r)
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	// The entry whose read failed must not be reported as a chart that was read.
	assert.Contains(t, err.Error(), "could not be read")
	assert.NotContains(t, err.Error(), `is "" version ""`)
}

func TestPullFromImageIndexUnreadableNamedVersionIsNotDescribed(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	titleOnly := alpha.indexDescriptor()
	delete(titleOnly.Annotations, ocispec.AnnotationVersion)
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// alpha announces its name but not its version, so the version has to be
		// fetched, and that fetch is the one that fails. beta keeps the candidate
		// count above one so selection reaches the name filter.
		Manifests: []ocispec.Descriptor{titleOnly, beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer base.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, alpha.manifestDigest.Encoded()) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		base.Config.Handler.ServeHTTP(w, r)
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/alpha:2.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not be read")
	// Calling it readable and naming its version are both claims about an entry
	// whose read is what failed.
	assert.NotContains(t, err.Error(), "the only readable chart")
	assert.NotContains(t, err.Error(), `is version ""`)
}

func TestPullFromImageIndexRepeatedEntry(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	desc := alpha.indexDescriptor()
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// The same manifest listed twice is one chart, not a choice between two.
		Manifests: []ocispec.Descriptor{desc, desc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexTwoChartsSameNameAndVersion(t *testing.T) {
	// Re-packaging a chart gives the archive a new modification time, which push
	// records as the creation annotation, so two builds of one chart version are
	// distinct manifests with identical identities.
	first := newTestChart("app", "1.0.0", "first")
	second := newTestChart("app", "1.0.0", "second")
	require.NotEqual(t, first.manifestDigest, second.manifestDigest)

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{first.indexDescriptor(), second.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, first, second)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// Name and version cannot separate them, so the error has to fall back to what
	// can: the digests, which here genuinely differ.
	_, err = client.Pull(host + "/testrepo/app:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "told apart by digest")
	assert.Contains(t, err.Error(), first.manifestDigest.String())
	assert.Contains(t, err.Error(), second.manifestDigest.String())
}

func TestPullGenericWithoutSelectors(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{alpha.indexDescriptor(), beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// PullGeneric is exported and a caller may pass no selectors at all. Two charts
	// and nothing to choose between them has to be reported, not guessed.
	_, err = client.Generic().PullGeneric(host+"/testrepo/anything:1.0.0", GenericPullOptions{
		AllowedMediaTypes: []string{ocispec.MediaTypeImageIndex, ocispec.MediaTypeImageManifest, ConfigMediaType, ChartLayerMediaType},
		ArtifactType:      ChartArtifactType,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no chart name was given to disambiguate")
}

func TestPullGenericWithoutIdentityParser(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Neither entry announces an identity, so without a parser there is nothing
		// left to match on and the requested name cannot be found.
		Manifests: []ocispec.Descriptor{alpha.declaredWithoutAnnotations(), beta.declaredWithoutAnnotations()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// PullGeneric is exported and ParseIdentity is optional: a caller that omits it
	// gets annotation-only matching, not a panic and not a config read.
	_, err = client.Generic().PullGeneric(host+"/testrepo/alpha:1.0.0", GenericPullOptions{
		AllowedMediaTypes: []string{ocispec.MediaTypeImageManifest, ConfigMediaType, ChartLayerMediaType},
		ArtifactType:      ChartArtifactType,
		Selectors:         map[string]string{ocispec.AnnotationTitle: "alpha"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `none is named "alpha"`)
}

func TestDescribeCandidates(t *testing.T) {
	d := func(s string) ocispec.Descriptor { return ocispec.Descriptor{Digest: digest.Digest(s)} }
	errFixture := errors.New("not found")
	sha := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	for _, tc := range []struct {
		name string
		in   []chartCandidate
		want string
	}{
		{"name and version", []chartCandidate{{desc: d(sha), name: "app", version: "1.0.0"}}, "app:1.0.0"},
		{"name only", []chartCandidate{{desc: d(sha), name: "app"}}, "app"},
		{"no identity read at all", []chartCandidate{{desc: d(sha)}}, sha},
		{"identity read failed", []chartCandidate{{desc: d(sha), idErr: errFixture}}, sha + " (identity unreadable)"},
		{"version read failed", []chartCandidate{{desc: d(sha), name: "app", idErr: errFixture}}, "app (version unreadable)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, describeCandidates(tc.in))
		})
	}
}

func TestPullFromImageIndexAmbiguousWithoutVersion(t *testing.T) {
	v1 := newTestChart("app", "1.0.0")
	v2 := newTestChart("app", "2.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{v1.indexDescriptor(), v2.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, v1, v2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// A digest-pinned reference carries no tag, so no version reaches selection and
	// two same-named charts cannot be told apart. That has to be said, not guessed.
	_, err = client.Pull(host + "/testrepo/app@" + indexDigest.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify a version")
	assert.Contains(t, err.Error(), "app:1.0.0")
	assert.Contains(t, err.Error(), "app:2.0.0")
}

func TestPullFromImageIndexReportsUnreadableEntries(t *testing.T) {
	alpha := newTestChart("alpha", "1.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{alpha.legacyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	// Serve the index but not the manifest it points at, so the only candidate the
	// fallback pass has cannot be read.
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/v2/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(p, indexDigest.Encoded()),
			strings.Contains(p, "/manifests/") && !strings.Contains(p, "sha256:"):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
			w.Header().Set("Docker-Content-Digest", indexDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	// "found nothing" and "could not read what was there" are different answers.
	assert.Contains(t, err.Error(), "could not be read")
	assert.Contains(t, err.Error(), alpha.manifestDigest.String())
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
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, v1, v2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assertSelected(t, v2, result)
}

func TestPullFromImageIndexSelectsByVersionWithoutVersionAnnotation(t *testing.T) {
	v1 := newTestChart("app", "1.0.0")
	v2 := newTestChart("app", "2.0.0")
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		// Both entries name the chart and neither carries a version annotation,
		// so the tie can only be broken on the version in each chart's config.
		Manifests: []ocispec.Descriptor{v1.titleOnlyIndexDescriptor(), v2.titleOnlyIndexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, v1, v2)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assertSelected(t, v2, result)
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
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/beta:1.0.0")
	require.NoError(t, err)
	assertSelected(t, beta, result)
}

func TestPullFromImageIndexNestedIndexIsNotACandidate(t *testing.T) {
	// An index entry may itself be an index, and nothing stops it from declaring the
	// chart artifact type and the chart's identity. Taken as a candidate it would be
	// handed to a copy that cannot root itself at an index; the chart it points at is
	// what the pull has to come back with.
	alpha := newTestChart("alpha", "1.0.0")

	innerDesc, innerBlob := indexHolding(alpha.indexDescriptor())
	innerDesc.ArtifactType = ChartArtifactType
	innerDesc.Annotations = map[string]string{
		ocispec.AnnotationTitle:   alpha.name,
		ocispec.AnnotationVersion: alpha.version,
	}

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{innerDesc, alpha.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	charts := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer charts.Close()
	s := serveAlso(charts, map[digest.Digest]rawBlob{innerDesc.Digest: innerBlob})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexFindsRequestedChartInsideNestedIndex(t *testing.T) {
	// The requested chart is reachable only through an entry that is an index. Left
	// unsearched it is a chart that is not there, and the chart beside it answers in
	// its place.
	alpha := newTestChart("alpha", "1.0.0")
	beta := newTestChart("beta", "1.0.0")

	innerDesc, innerBlob := indexHolding(alpha.indexDescriptor())

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{innerDesc, beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	charts := serveMultiChartIndex(indexBytes, indexDigest, alpha, beta)
	defer charts.Close()
	s := serveAlso(charts, map[digest.Digest]rawBlob{innerDesc.Digest: innerBlob})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexFindsRequestedVersionInsideNestedIndex(t *testing.T) {
	// Same search, one axis over: the requested version is the one nested away, and
	// the version left in plain sight carries the requested name.
	wanted := newTestChart("app", "2.0.0")
	older := newTestChart("app", "1.0.0")
	other := newTestChart("other", "1.0.0")

	innerDesc, innerBlob := indexHolding(wanted.indexDescriptor())

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{innerDesc, older.indexDescriptor(), other.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	charts := serveMultiChartIndex(indexBytes, indexDigest, wanted, older, other)
	defer charts.Close()
	s := serveAlso(charts, map[digest.Digest]rawBlob{innerDesc.Digest: innerBlob})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:2.0.0")
	require.NoError(t, err)
	assertSelected(t, wanted, result)
}

func TestPullFromImageIndexUnreadableNestedIndexIsNotAnAbsentChart(t *testing.T) {
	// A nested index that cannot be fetched is the case the search cannot complete.
	// The chart beside it is then not the only chart in the index, it is the only one
	// that could be read, and answering with it would state a fact about the read.
	beta := newTestChart("beta", "1.0.0")

	unreachable := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
		Digest:    digest.FromString("unreachable"),
		Size:      2,
	}
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{unreachable, beta.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, beta)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not be read")
}

func TestPullFromImageIndexChartNamedUnlikeItsRepositoryBesideNestedIndex(t *testing.T) {
	// Publishing a chart under a repository named after something else is legal, and
	// the sole chart in an index answers whatever name was asked for. A multi-arch
	// image sitting beside it is an index, and searching it must not turn the name
	// this pull never knew into grounds for refusing the chart.
	chart := newTestChart("testchart", "1.0.0")

	imageConfig := []byte(`{"architecture":"amd64","os":"linux"}`)
	imageConfigDigest := digest.FromBytes(imageConfig)
	imageManifest, _ := json.Marshal(ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    imageConfigDigest,
			Size:      int64(len(imageConfig)),
		},
	})
	imageDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(imageManifest),
		Size:      int64(len(imageManifest)),
		Platform:  &ocispec.Platform{OS: "linux", Architecture: "amd64"},
	}
	innerDesc, innerBlob := indexHolding(imageDesc)

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{chart.indexDescriptor(), innerDesc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	charts := serveMultiChartIndex(indexBytes, indexDigest, chart)
	defer charts.Close()
	s := serveAlso(charts, map[digest.Digest]rawBlob{
		innerDesc.Digest: innerBlob,
		imageDesc.Digest: {mediaType: ocispec.MediaTypeImageManifest, data: imageManifest},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/multichart:1.0.0")
	require.NoError(t, err)
	assertSelected(t, chart, result)
}

func TestPullFromImageIndexSelectsVersionCarryingBuildMetadata(t *testing.T) {
	// A version with build metadata reaches the registry with the plus sign encoded
	// as an underscore, while the chart's own annotation keeps the plus, so selection
	// has to undo the encoding before the two can be compared.
	withMetadata := newTestChart("app", "1.0.0+build")
	plain := newTestChart("app", "2.0.0")

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{plain.indexDescriptor(), withMetadata.indexDescriptor()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, plain, withMetadata)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/app:1.0.0+build")
	require.NoError(t, err)
	assertSelected(t, withMetadata, result)
}

// nestedChain wraps desc in levels indexes, each holding the one below, and returns
// the outermost of them together with every body the chain needs served.
func nestedChain(desc ocispec.Descriptor, levels int) (ocispec.Descriptor, map[digest.Digest]rawBlob) {
	blobs := map[digest.Digest]rawBlob{}
	for range levels {
		wrapper, blob := indexHolding(desc)
		blobs[wrapper.Digest] = blob
		desc = wrapper
	}
	return desc, blobs
}

func TestPullFromImageIndexStopsAtTheNestingLimit(t *testing.T) {
	// The chart is reachable only by following indexes, so how deep the search goes
	// is the only thing that decides whether it is found. At the limit it is; one
	// index further down it is not, and the pull says which entry it left unread
	// rather than reporting a chart that is not there.
	alpha := newTestChart("alpha", "1.0.0")

	pull := func(t *testing.T, levels int) (*PullResult, error) {
		t.Helper()
		outermost, chain := nestedChain(alpha.indexDescriptor(), levels)
		index := ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: []ocispec.Descriptor{outermost},
		}
		indexBytes, _ := json.Marshal(index)
		indexDigest := digest.FromBytes(indexBytes)

		charts := serveMultiChartIndex(indexBytes, indexDigest, alpha)
		defer charts.Close()
		s := serveAlso(charts, chain)
		defer s.Close()
		u, _ := url.Parse(s.URL)

		client, err := NewClient(ClientOptPlainHTTP())
		require.NoError(t, err)
		return client.Pull("localhost:" + u.Port() + "/testrepo/alpha:1.0.0")
	}

	result, err := pull(t, maxIndexDepth)
	require.NoError(t, err)
	assertSelected(t, alpha, result)

	_, err = pull(t, maxIndexDepth+1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "indexes deep and was not searched")
}

func TestPullFromImageIndexEntryDeclaringTheWrongTypeStillResolves(t *testing.T) {
	// A builder that sets artifactType to something other than the chart type has
	// broken the descriptor the same way one that omits it has, and in both cases the
	// config the manifest carries is where the artifact says what it is.
	alpha := newTestChart("alpha", "1.0.0")
	desc := alpha.indexDescriptor()
	desc.ArtifactType = ocispec.MediaTypeImageManifest

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{desc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexUnreadableNeighbourIsConfirmedAgainstOrNamed(t *testing.T) {
	// An entry declaring a type of its own is read for the config its manifest
	// carries, so a registry that will not serve it leaves the pass incomplete. What
	// the sole chart is then worth depends on whether its identity can be confirmed
	// against the reference: confirmed it answers, unconfirmed the entry that could
	// not be read is named, since it is the one that might have held the request.
	unreadable := ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		Digest:       digest.FromString("neighbour the registry will not serve"),
		Size:         12,
		ArtifactType: "application/vnd.oci.image.config.v1+json",
	}

	pull := func(t *testing.T, repository string) (*PullResult, error) {
		t.Helper()
		alpha := newTestChart("alpha", "1.0.0")
		index := ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			// Without annotations the declared entry announces no identity, so the
			// entries that did not match are read and the neighbour is reached.
			Manifests: []ocispec.Descriptor{alpha.declaredWithoutAnnotations(), unreadable},
		}
		indexBytes, _ := json.Marshal(index)
		indexDigest := digest.FromBytes(indexBytes)

		s := serveMultiChartIndex(indexBytes, indexDigest, alpha)
		defer s.Close()
		u, _ := url.Parse(s.URL)

		client, err := NewClient(ClientOptPlainHTTP())
		require.NoError(t, err)
		return client.Pull("localhost:" + u.Port() + "/" + repository + ":1.0.0")
	}

	t.Run("identity confirmed", func(t *testing.T) {
		result, err := pull(t, "testrepo/alpha")
		require.NoError(t, err)
		assertSelected(t, newTestChart("alpha", "1.0.0"), result)
	})

	t.Run("identity not confirmed", func(t *testing.T) {
		_, err := pull(t, "testrepo/somethingelse")
		require.Error(t, err)
		assert.Contains(t, err.Error(), unreadable.Digest.String())
	})
}

func TestPullFromImageIndexRepeatedEntryIsReadOnce(t *testing.T) {
	// An index may list the same manifest twice, and the repeat is not read again.
	// The count in the message says how many entries were looked at, so it has to
	// come from the reading rather than from the length of the queue.
	image, _ := json.Marshal(ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.FromString("image config"),
			Size:      10,
		},
	})
	imageDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(image),
		Size:      int64(len(image)),
	}

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{imageDesc, imageDesc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest)
	defer base.Close()
	s := serveAlso(base, map[digest.Digest]rawBlob{
		imageDesc.Digest: {mediaType: ocispec.MediaTypeImageManifest, data: image},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull("localhost:" + u.Port() + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the manifest of the one other entry was read")
}

func TestPullFromImageIndexFindsChartInsideDockerManifestList(t *testing.T) {
	// Docker Hub and older buildx write a manifest list where the OCI spec writes an
	// index. It lists manifests the same way, so an entry carrying it is searched the
	// same way; treated as an ordinary manifest it holds no config, and the chart
	// inside it becomes a chart the index is said not to hold.
	const dockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	alpha := newTestChart("alpha", "1.0.0")

	inner, _ := json.Marshal(ocispec.Index{
		MediaType: dockerManifestList,
		Manifests: []ocispec.Descriptor{alpha.indexDescriptor()},
	})
	innerDigest := digest.FromBytes(inner)

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{{
			MediaType: dockerManifestList,
			Digest:    innerDigest,
			Size:      int64(len(inner)),
		}},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer base.Close()
	s := serveAlso(base, map[digest.Digest]rawBlob{
		innerDigest: {mediaType: dockerManifestList, data: inner},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull("localhost:" + u.Port() + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexOversizedEntryIsNotRead(t *testing.T) {
	// The size on an index entry is the index's claim about content the registry
	// serves. A manifest that large is not a manifest, so the entry is left unread
	// and named rather than streamed into memory to be discarded.
	alpha := newTestChart("alpha", "1.0.0")

	oversized := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromString("oversized neighbour"),
		Size:      maxEntryBytes + 1,
	}
	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{alpha.declaredWithoutAnnotations(), oversized},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	// The oversized entry is served, so a pull that reads it succeeds in reading it;
	// only the size gate keeps the body out.
	served := make([]byte, 0)
	s := serveAlso(serveMultiChartIndex(indexBytes, indexDigest, alpha), map[digest.Digest]rawBlob{
		oversized.Digest: {mediaType: ocispec.MediaTypeImageManifest, data: served},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	// Asked for by its own name, the chart still resolves: the unread entry makes the
	// pass incomplete, and the identity of what is left is confirmed against the
	// reference before it answers.
	result, err := client.Pull(host + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)

	// Asked for by another name, the entry that was not read is named, since it is
	// the one that might have held the chart.
	_, err = client.Pull(host + "/testrepo/somethingelse:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than a manifest is")
	assert.Contains(t, err.Error(), oversized.Digest.String())
}

func TestPullFromImageIndexEntryHoldingSomethingElseIsNotSilentlyDropped(t *testing.T) {
	// An index body parses cleanly into a manifest and a manifest body parses cleanly
	// into an index, because neither shape has a field the other rejects. An entry
	// whose body does not hold what it declared is therefore read as neither, and
	// saying so is what keeps the chart beside it from answering unchallenged.
	beta := newTestChart("beta", "1.0.0")
	foo := newTestChart("foo", "1.0.0")

	// An index body, listed under a manifest media type.
	mislabelled, _ := json.Marshal(ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{foo.indexDescriptor()},
	})
	mislabelledDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(mislabelled),
		Size:      int64(len(mislabelled)),
	}

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{mislabelledDesc, beta.declaredWithoutAnnotations()},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	s := serveAlso(serveMultiChartIndex(indexBytes, indexDigest, beta, foo), map[digest.Digest]rawBlob{
		mislabelledDesc.Digest: {mediaType: ocispec.MediaTypeImageManifest, data: mislabelled},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	host := "localhost:" + u.Port()

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull(host + "/testrepo/foo:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), mislabelledDesc.Digest.String())
}

func TestPullFromImageIndexChartWrittenToTheDockerSchema(t *testing.T) {
	// A chart copied between registries by tooling that writes the Docker schema
	// carries the Docker manifest media type while describing the same config and
	// layers. Selecting it and then refusing to copy it fails the pull on a blob
	// nothing stored, so the type the copy accepts has to cover what selection picks.
	alpha := newTestChart("alpha", "1.0.0")
	desc := alpha.indexDescriptor()
	desc.MediaType = dockerManifestMediaType

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{desc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	// The registry answers for that manifest with the type the descriptor declares,
	// which is what a registry holding a converted chart does.
	base := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer base.Close()
	s := serveAlso(base, map[digest.Digest]rawBlob{
		alpha.manifestDigest: {mediaType: dockerManifestMediaType, data: alpha.manifestBytes},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	result, err := client.Pull("localhost:" + u.Port() + "/testrepo/alpha:1.0.0")
	require.NoError(t, err)
	assertSelected(t, alpha, result)
}

func TestPullFromImageIndexChartTheCopyCannotStoreIsNamed(t *testing.T) {
	// Selection matches on the artifact type and the config, neither of which says
	// how the manifest itself is written. A chart written in a form this pull does
	// not copy has to be named by selection, which knows what it picked and why,
	// rather than left to surface as a blob the copy never stored.
	alpha := newTestChart("alpha", "1.0.0")
	desc := alpha.indexDescriptor()
	const artifactManifest = "application/vnd.oci.artifact.manifest.v1+json"
	desc.MediaType = artifactManifest

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{desc},
	}
	indexBytes, _ := json.Marshal(index)
	indexDigest := digest.FromBytes(indexBytes)

	base := serveMultiChartIndex(indexBytes, indexDigest, alpha)
	defer base.Close()
	s := serveAlso(base, map[digest.Digest]rawBlob{
		alpha.manifestDigest: {mediaType: artifactManifest, data: alpha.manifestBytes},
	})
	defer s.Close()
	u, _ := url.Parse(s.URL)

	client, err := NewClient(ClientOptPlainHTTP())
	require.NoError(t, err)

	_, err = client.Pull("localhost:" + u.Port() + "/testrepo/alpha:1.0.0")
	require.Error(t, err)
	// Named by selection, which knows the entry and the reason. Left to the copy, the
	// same shape comes back as a count of descriptors that were not collected, which
	// names neither the entry that was picked nor why nothing came of it.
	assert.Contains(t, err.Error(), "which this pull does not accept")
	assert.Contains(t, err.Error(), artifactManifest)
	assert.NotContains(t, err.Error(), "not found")
}
