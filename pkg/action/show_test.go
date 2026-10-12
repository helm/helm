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
	require.NoError(t, err)

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
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
}

func TestShowNoValues(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	client.chart = new(chart.Chart)

	// Regression tests for missing values. See issue #1024.
	client.OutputFormat = ShowValues
	output, err := client.Run("")
	require.NoError(t, err)

	assert.Empty(t, output, "expected empty values buffer, got %s", output)
}

func TestShowValuesByJsonPathFormat(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowValues, config)
	client.JSONPathTemplate = "{$.nestedKey.simpleKey}"
	client.chart = buildChart(withSampleValues())
	output, err := client.Run("")
	require.NoError(t, err)
	expect := "simpleValue"
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
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
	require.NoError(t, err)

	expect := `---
foo

---
bar

---
baz

`
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
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
	require.NoError(t, err)

	expect := `name: alpine

---
foo

---
bar

`
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
}

func TestShowSetRegistryClient(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)

	registryClient := &registry.Client{}
	client.SetRegistryClient(registryClient)
	assert.Equal(t, registryClient, client.registryClient)
}

func TestShowChartWithOCIAnnotations(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowChart, config)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine", Version: "0.1.0"},
	}
	client.ociAnnotations = map[string]string{
		"org.opencontainers.image.title":   "alpine",
		"org.opencontainers.image.created": "1977-09-02T22:04:05Z",
	}

	output, err := client.Run("")
	require.NoError(t, err)

	expect := `name: alpine
version: 0.1.0
ociAnnotations:
  org.opencontainers.image.created: "1977-09-02T22:04:05Z"
  org.opencontainers.image.title: alpine

`
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
}

func TestShowAllWithOCIAnnotations(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "README.md", ModTime: modTime, Data: []byte("README\n")},
		},
		Raw: []*common.File{
			{Name: "values.yaml", ModTime: modTime, Data: []byte("VALUES\n")},
		},
		Values: map[string]any{},
	}
	client.ociAnnotations = map[string]string{
		"org.opencontainers.image.title": "alpine",
	}

	output, err := client.Run("")
	require.NoError(t, err)

	expect := `name: alpine
ociAnnotations:
  org.opencontainers.image.title: alpine

---
VALUES

---
README

`
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
}

func TestShowValuesIgnoresOCIAnnotations(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowValues, config)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Raw: []*common.File{
			{Name: "values.yaml", ModTime: time.Now(), Data: []byte("VALUES\n")},
		},
		Values: map[string]any{},
	}
	client.ociAnnotations = map[string]string{
		"org.opencontainers.image.title": "alpine",
	}

	output, err := client.Run("")
	require.NoError(t, err)
	assert.Equal(t, "VALUES\n\n", output)
}

func TestShowLoadOCIAnnotations(t *testing.T) {
	config := actionConfigFixture(t)

	t.Run("no-op when not requested", func(t *testing.T) {
		client := NewShow(ShowChart, config)
		require.NoError(t, client.LoadOCIAnnotations("oci://localhost:5000/repo/chart:1.0.0"))
		assert.Nil(t, client.ociAnnotations)
	})

	t.Run("no-op for non-OCI references", func(t *testing.T) {
		client := NewShow(ShowChart, config)
		client.IncludeOCIAnnotations = true
		require.NoError(t, client.LoadOCIAnnotations("testdata/charts/alpine"))
		assert.Nil(t, client.ociAnnotations)
	})

	t.Run("error without registry client", func(t *testing.T) {
		client := NewShow(ShowChart, config)
		client.IncludeOCIAnnotations = true
		client.SetRegistryClient(nil)
		err := client.LoadOCIAnnotations("oci://localhost:5000/repo/chart:1.0.0")
		require.ErrorContains(t, err, "missing registry client")
	})
}
