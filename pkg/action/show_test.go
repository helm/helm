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

package action

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/registry"
)

func TestShow(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "README.md", ModTime: modTime, Data: []byte("README\n")},
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
			{Name: "crds/baz.yaml", ModTime: modTime, Data: []byte("baz\n")},
		},
		Raw: []*common.File{
			{Name: "values.yaml", ModTime: modTime, Data: []byte("VALUES\n")},
		},
		Values: map[string]any{},
	}

	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	expect := `name: alpine

---
VALUES

---
README

---
foo

---
bar

---
baz

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowNoValues(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	client.chart = new(chart.Chart)

	// Regression tests for missing values. See issue #1024.
	client.OutputFormat = ShowValues
	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	if len(output) != 0 {
		t.Errorf("expected empty values buffer, got %s", output)
	}
}

func TestShowValuesByJsonPathFormat(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowValues, config)
	client.JSONPathTemplate = "{$.nestedKey.simpleKey}"
	client.chart = buildChart(withSampleValues())
	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}
	expect := "simpleValue"
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowCRDs(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowCRDs, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
			{Name: "crds/baz.yaml", ModTime: modTime, Data: []byte("baz\n")},
		},
	}

	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	expect := `---
foo

---
bar

---
baz

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowNoReadme(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
		},
	}

	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	expect := `name: alpine

---
foo

---
bar

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowSetRegistryClient(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)

	registryClient := &registry.Client{}
	client.SetRegistryClient(registryClient)
	assert.Equal(t, registryClient, client.registryClient)
}

func TestShowOCIAnnotations(t *testing.T) {
	const manifestJSON = `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {
			"mediaType": "application/vnd.cncf.helm.config.v1+json",
			"digest": "sha256:abc123",
			"size": 100
		},
		"layers": [],
		"annotations": {
			"org.opencontainers.image.created": "2025-04-11T20:12:25Z"
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(manifestJSON))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	registryClient, err := registry.NewClient(
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptPlainHTTP(),
	)
	require.NoError(t, err)

	config := actionConfigFixture(t)
	client := NewShow(ShowChart, config)
	client.SetRegistryClient(registryClient)
	client.OCIRef = host + "/test/chart:1.0.0"
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test-chart", Version: "1.0.0"},
	}

	output, err := client.Run("")
	require.NoError(t, err)

	assert.Contains(t, output, "---")
	assert.Contains(t, output, "org.opencontainers.image.created")
	assert.Contains(t, output, "2025-04-11T20:12:25Z")
}

func TestShowOCIAnnotationsNoRegistry(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowChart, config)
	// OCIRef set but no registry client - should not fail, just omit annotations
	client.OCIRef = "host/test/chart:1.0.0"
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test-chart", Version: "1.0.0"},
	}

	output, err := client.Run("")
	require.NoError(t, err)
	// No annotations section, no "---" separator
	assert.NotContains(t, output, "---")
}

func TestShowOCIAnnotationsResolvesVersion(t *testing.T) {
	const manifestJSON = `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {
			"mediaType": "application/vnd.cncf.helm.config.v1+json",
			"digest": "sha256:abc123",
			"size": 100
		},
		"layers": [],
		"annotations": {
			"org.opencontainers.image.created": "2025-04-11T20:12:25Z"
		}
	}`

	var requestedManifest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "/manifests/") {
			requestedManifest = r.URL.Path
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(manifestJSON))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	registryClient, err := registry.NewClient(
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptPlainHTTP(),
	)
	require.NoError(t, err)

	config := actionConfigFixture(t)
	client := NewShow(ShowChart, config)
	client.SetRegistryClient(registryClient)
	// OCIRef has no explicit tag; the concrete tag is taken from the chart version.
	client.OCIRef = host + "/test/chart"
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test-chart", Version: "1.0.0"},
	}

	output, err := client.Run("")
	require.NoError(t, err)

	assert.Contains(t, output, "org.opencontainers.image.created")
	assert.Contains(t, output, "2025-04-11T20:12:25Z")
	assert.Contains(t, requestedManifest, "/manifests/1.0.0")
}
