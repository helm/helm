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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
)

const testfile = "testdata/chartfiletest.yaml"

func TestLoadChartfile(t *testing.T) {
	f, err := LoadChartfile(testfile)
	require.NoErrorf(t, err, "Failed to open %s", testfile)
	verifyChartfile(t, f, "frobnitz")
}

func verifyChartfile(t *testing.T, f *chart.Metadata, name string) {
	t.Helper()
	require.NotNil(t, f, "Failed verifyChartfile because f is nil")
	assert.Equal(t, name, f.Name, "Expected %s, got %s", name, f.Name)
	assert.Equal(t, "This is a frobnitz.", f.Description, "Unexpected description %q", f.Description)
	assert.Equal(t, "1.2.3", f.Version, "Unexpected version %q", f.Version)
	assert.Len(t, f.Maintainers, 2, "Expected 2 maintainers, got %d", len(f.Maintainers))
	assert.Equal(t, "The Helm Team", f.Maintainers[0].Name, "Unexpected maintainer name.")
	assert.Equal(t, "nobody@example.com", f.Maintainers[1].Email, "Unexpected maintainer email.")
	require.Len(t, f.Sources, 1, "Unexpected number of sources")
	assert.Equal(t, "https://example.com/foo/bar", f.Sources[0], "Expected https://example.com/foo/bar, got %s", f.Sources)
	assert.Equal(t, "http://example.com", f.Home, "Unexpected home.")
	assert.Equal(t, "https://example.com/64x64.png", f.Icon, "Unexpected icon: %q", f.Icon)
	require.Len(t, f.Keywords, 3, "Unexpected keywords")
	require.Len(t, f.Annotations, 2, "Unexpected annotations")

	assert.Equal(t, "extravalue", f.Annotations["extrakey"])
	assert.Equal(t, "anothervalue", f.Annotations["anotherkey"])

	kk := []string{"frobnitz", "sprocket", "dodad"}
	for i, k := range f.Keywords {
		assert.Equal(t, kk[i], k, "Expected %q, got %q", kk[i], k)
	}
}

func TestIsChartDir(t *testing.T) {
	validChartDir, err := IsChartDir("testdata/frobnitz")
	require.NoError(t, err, "while reading chart-directory")
	require.True(t, validChartDir, "expected valid chart directory")
	validChartDir, err = IsChartDir("testdata")
	require.Error(t, err)
	require.False(t, validChartDir, "expected invalid chart directory")
}
