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

package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/repo/v1"
)

func TestRepoIndexCmd(t *testing.T) {
	dir := t.TempDir()

	comp := filepath.Join(dir, "compressedchart-0.1.0.tgz")
	require.NoError(t, linkOrCopy("testdata/testcharts/compressedchart-0.1.0.tgz", comp))
	comp2 := filepath.Join(dir, "compressedchart-0.2.0.tgz")
	require.NoError(t, linkOrCopy("testdata/testcharts/compressedchart-0.2.0.tgz", comp2))

	buf := bytes.NewBuffer(nil)
	c := newRepoIndexCmd(buf)

	require.NoError(t, c.RunE(c, []string{dir}))

	destIndex := filepath.Join(dir, "index.yaml")

	index, err := repo.LoadIndexFile(destIndex)
	require.NoError(t, err)

	require.Len(t, index.Entries, 1, "expected 1 entry, got %d: %#v", len(index.Entries), index.Entries)

	vs := index.Entries["compressedchart"]
	require.Len(t, vs, 2, "expected 2 versions")

	expectedVersion := "0.2.0"
	assert.Equal(t, expectedVersion, vs[0].Version, "expected %q, got %q", expectedVersion, vs[0].Version)

	b, err := os.ReadFile(destIndex)
	require.NoError(t, err)
	assert.False(t, json.Valid(b), "did not expect index file to be valid json")

	// Test with `--json`

	c.ParseFlags([]string{"--json", "true"})
	require.NoError(t, c.RunE(c, []string{dir}))

	b, err = os.ReadFile(destIndex)
	require.NoError(t, err)
	assert.True(t, json.Valid(b), "index file is not valid json")

	// Test with `--merge`

	// Remove first two charts.
	require.NoError(t, os.Remove(comp))
	require.NoError(t, os.Remove(comp2))
	// Add a new chart and a new version of an existing chart
	require.NoError(t, linkOrCopy("testdata/testcharts/reqtest-0.1.0.tgz", filepath.Join(dir, "reqtest-0.1.0.tgz")))
	require.NoError(t, linkOrCopy("testdata/testcharts/compressedchart-0.3.0.tgz", filepath.Join(dir, "compressedchart-0.3.0.tgz")))

	c.ParseFlags([]string{"--merge", destIndex})
	require.NoError(t, c.RunE(c, []string{dir}))

	index, err = repo.LoadIndexFile(destIndex)
	require.NoError(t, err)

	assert.Len(t, index.Entries, 2, "expected 2 entries, got %d: %#v", len(index.Entries), index.Entries)

	vs = index.Entries["compressedchart"]
	assert.Len(t, vs, 3, "expected 3 versions, got %d: %#v", len(vs), vs)

	expectedVersion = "0.3.0"
	assert.Equal(t, expectedVersion, vs[0].Version, "expected %q, got %q", expectedVersion, vs[0].Version)

	// test that index.yaml gets generated on merge even when it doesn't exist
	require.NoError(t, os.Remove(destIndex))

	c.ParseFlags([]string{"--merge", destIndex})
	require.NoError(t, c.RunE(c, []string{dir}))

	index, err = repo.LoadIndexFile(destIndex)
	require.NoError(t, err)

	// verify it didn't create an empty index.yaml and the merged happened
	assert.Len(t, index.Entries, 2, "expected 2 entries, got %d: %#v", len(index.Entries), index.Entries)

	vs = index.Entries["compressedchart"]
	assert.Len(t, vs, 1, "expected 1 versions, got %d: %#v", len(vs), vs)

	expectedVersion = "0.3.0"
	assert.Equal(t, expectedVersion, vs[0].Version, "expected %q, got %q", expectedVersion, vs[0].Version)
}

func linkOrCopy(source, target string) error {
	if err := os.Link(source, target); err != nil {
		return copyFile(source, target)
	}

	return nil
}

func copyFile(dst, src string) error {
	i, err := os.Open(dst)
	if err != nil {
		return err
	}
	defer i.Close()

	o, err := os.Create(src)
	if err != nil {
		return err
	}
	defer o.Close()

	_, err = io.Copy(o, i)

	return err
}

func TestRepoIndexFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo index", true)
	checkFileCompletion(t, "repo index mydir", false)
}
