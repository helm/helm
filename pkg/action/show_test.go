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
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/registry"
)

func TestShowRunLoadsEachChart(t *testing.T) {
	const archiveName = "compressedchart-0.1.0.tgz"
	archive := filepath.Join(t.TempDir(), archiveName)
	data, err := os.ReadFile(filepath.Join("testdata", "charts", archiveName))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(archive, data, 0o600))

	client := NewShow(ShowChart, actionConfigFixture(t))
	for _, tc := range []struct{ path, name string }{
		{"testdata/charts/decompressedchart", "decompressedchart"},
		{archive, "compressedchart"},
	} {
		out, err := client.Run(tc.path)
		require.NoError(t, err)
		var metadata chart.Metadata
		require.NoError(t, yaml.Unmarshal([]byte(out), &metadata))
		assert.Equal(t, tc.name, metadata.Name)
	}

	require.NoError(t, os.Remove(archive))
	out, err := client.Run(archive)
	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Empty(t, out)
}

func TestShow(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	modTime := time.Now()
	chrt := &chart.Chart{
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

	output, err := client.renderChart(chrt)
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

	// Regression tests for missing values. See issue #1024.
	client.OutputFormat = ShowValues
	output, err := client.renderChart(new(chart.Chart))
	require.NoError(t, err)

	assert.Empty(t, output, "expected empty values buffer, got %s", output)
}

func TestShowValuesByJsonPathFormat(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowValues, config)
	client.JSONPathTemplate = "{$.nestedKey.simpleKey}"
	output, err := client.renderChart(buildChart(withSampleValues()))
	require.NoError(t, err)
	expect := "simpleValue"
	assert.Equal(t, expect, output, "Expected\n%q\nGot\n%q\n", expect, output)
}

func TestShowCRDs(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowCRDs, config)
	modTime := time.Now()
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
			{Name: "crds/baz.yaml", ModTime: modTime, Data: []byte("baz\n")},
		},
	}

	output, err := client.renderChart(chrt)
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
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
		},
	}

	output, err := client.renderChart(chrt)
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
