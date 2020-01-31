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

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/repo"
)

func TestRepoIndexCmd(t *testing.T) {
	// t.Run("helm repo index command", func(t *testing.T) {
	dir := ensure.TempDir(t)

	comp := filepath.Join(dir, "compressedchart-0.1.0.tgz")
	if err := linkOrCopy("testdata/testcharts/compressedchart-0.1.0.tgz", comp); err != nil {
		t.Fatal(err)
	}
	comp2 := filepath.Join(dir, "compressedchart-0.2.0.tgz")
	if err := linkOrCopy("testdata/testcharts/compressedchart-0.2.0.tgz", comp2); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)
	c := newRepoIndexCmd(buf)
	expectedNumberOfEntries := 1
	expectedNumberOfVersions := 2

	if err := c.RunE(c, []string{dir}); err != nil {
		t.Error(err)
	}

	destIndex := filepath.Join(dir, "index.yaml")

	index, err := repo.LoadIndexFile(destIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(index.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entry, got %d: %#v",
			expectedNumberOfEntries, len(index.Entries), index.Entries)
	}

	vs := index.Entries["compressedchart"]
	if len(vs) != expectedNumberOfVersions {
		t.Errorf("expected %d versions, got %d: %#v",
			expectedNumberOfVersions, len(vs), vs)
	}

	expectedVersion := "0.2.0"
	if vs[0].Version != expectedVersion {
		t.Errorf("expected %q, got %q", expectedVersion, vs[0].Version)
	}

	// Test creation of index.json
	destJSONIndex := filepath.Join(dir, "index.json")

	jsonIndex, err := repo.LoadIndexJSONFile(destJSONIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(jsonIndex.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entry in json index, got %d: %#v",
			expectedNumberOfEntries, len(jsonIndex.Entries), jsonIndex.Entries)
	}

	versionsInJSONIndex := jsonIndex.Entries["compressedchart"]
	if len(versionsInJSONIndex) != expectedNumberOfVersions {
		t.Errorf("expected %d versions in json index, got %d: %#v",
			expectedNumberOfVersions, len(versionsInJSONIndex), versionsInJSONIndex)
	}

	expectedVersionInJSONIndex := "0.2.0"
	if versionsInJSONIndex[0].Version != expectedVersionInJSONIndex {
		t.Errorf("expected %q in json index, got %q",
			expectedVersionInJSONIndex, versionsInJSONIndex[0].Version)
	}

	// save a copy of index.json to test --merge-json-index later
	indexForMergeJSON := filepath.Join(dir, "indexForMerge.json")
	if err = copyFile(destJSONIndex, indexForMergeJSON); err != nil {
		t.Fatal(err)
	}

	// save a copy of index.yaml to test --merge now
	indexForMergeYAML := filepath.Join(dir, "indexForMerge.yaml")
	if err = copyFile(destIndex, indexForMergeYAML); err != nil {
		t.Fatal(err)
	}

	// Test with `--merge`
	// cleanup index.json and index.yaml as it's not needed. it will be created
	// by the command
	if err := os.Remove(destJSONIndex); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(destIndex); err != nil {
		t.Fatal(err)
	}
	// Remove first two charts.
	if err := os.Remove(comp); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(comp2); err != nil {
		t.Fatal(err)
	}
	// Add a new chart and a new version of an existing chart
	if err := linkOrCopy("testdata/testcharts/reqtest-0.1.0.tgz", filepath.Join(dir, "reqtest-0.1.0.tgz")); err != nil {
		t.Fatal(err)
	}
	if err := linkOrCopy("testdata/testcharts/compressedchart-0.3.0.tgz", filepath.Join(dir, "compressedchart-0.3.0.tgz")); err != nil {
		t.Fatal(err)
	}

	buf = bytes.NewBuffer(nil)
	c = newRepoIndexCmd(buf)
	expectedNumberOfEntries = 2
	expectedNumberOfVersions = 3

	if err = c.ParseFlags([]string{"--merge", indexForMergeYAML}); err != nil {
		t.Error(err)
	}
	if err := c.RunE(c, []string{dir}); err != nil {
		t.Error(err)
	}

	index, err = repo.LoadIndexFile(destIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(index.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entries, got %d: %#v",
			expectedNumberOfEntries, len(index.Entries), index.Entries)
	}

	vs = index.Entries["compressedchart"]
	if len(vs) != expectedNumberOfVersions {
		t.Errorf("expected %d versions, got %d: %#v",
			expectedNumberOfVersions, len(vs), vs)
	}

	expectedVersion = "0.3.0"
	if vs[0].Version != expectedVersion {
		t.Errorf("expected %q, got %q", expectedVersion, vs[0].Version)
	}

	jsonIndex, err = repo.LoadIndexJSONFile(destJSONIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(jsonIndex.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entry in json index, got %d: %#v",
			expectedNumberOfEntries, len(jsonIndex.Entries), jsonIndex.Entries)
	}

	versionsInJSONIndex = jsonIndex.Entries["compressedchart"]
	if len(versionsInJSONIndex) != expectedNumberOfVersions {
		t.Errorf("expected %d versions in json index, got %d: %#v",
			expectedNumberOfVersions, len(versionsInJSONIndex), versionsInJSONIndex)
	}

	expectedVersionInJSONIndex = "0.3.0"
	if versionsInJSONIndex[0].Version != expectedVersionInJSONIndex {
		t.Errorf("expected %q in json index, got %q",
			expectedVersionInJSONIndex, versionsInJSONIndex[0].Version)
	}

	// Test with `--merge-json-index`
	// cleanup index.yaml and index.json as it's not needed. it will be created
	// by the command
	if err := os.Remove(destIndex); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(destJSONIndex); err != nil {
		t.Fatal(err)
	}

	buf = bytes.NewBuffer(nil)
	c = newRepoIndexCmd(buf)
	expectedNumberOfEntries = 2
	expectedNumberOfVersions = 3

	if err = c.ParseFlags([]string{"--merge-json-index", indexForMergeJSON}); err != nil {
		t.Error(err)
	}
	if err := c.RunE(c, []string{dir}); err != nil {
		t.Error(err)
	}

	index, err = repo.LoadIndexFile(destIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(index.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entries, got %d: %#v",
			expectedNumberOfEntries, len(index.Entries), index.Entries)
	}

	vs = index.Entries["compressedchart"]
	if len(vs) != expectedNumberOfVersions {
		t.Errorf("expected %d versions, got %d: %#v",
			expectedNumberOfVersions, len(vs), vs)
	}

	expectedVersion = "0.3.0"
	if vs[0].Version != expectedVersion {
		t.Errorf("expected %q, got %q", expectedVersion, vs[0].Version)
	}

	jsonIndex, err = repo.LoadIndexJSONFile(destJSONIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(jsonIndex.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entry in json index, got %d: %#v",
			expectedNumberOfEntries, len(jsonIndex.Entries), jsonIndex.Entries)
	}

	versionsInJSONIndex = jsonIndex.Entries["compressedchart"]
	if len(versionsInJSONIndex) != expectedNumberOfVersions {
		t.Errorf("expected %d versions in json index, got %d: %#v",
			expectedNumberOfVersions, len(versionsInJSONIndex), versionsInJSONIndex)
	}

	expectedVersionInJSONIndex = "0.3.0"
	if versionsInJSONIndex[0].Version != expectedVersionInJSONIndex {
		t.Errorf("expected %q in json index, got %q",
			expectedVersionInJSONIndex, versionsInJSONIndex[0].Version)
	}

	// test that index.yaml and index.json gets generated on
	// merge even when given index.yaml file doesn't doesn't exist
	if err := os.Remove(destIndex); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(destJSONIndex); err != nil {
		t.Fatal(err)
	}

	buf = bytes.NewBuffer(nil)
	c = newRepoIndexCmd(buf)
	expectedNumberOfEntries = 2
	expectedNumberOfVersions = 1

	if err = c.ParseFlags([]string{"--merge", destIndex}); err != nil {
		t.Error(err)
	}
	if err := c.RunE(c, []string{dir}); err != nil {
		t.Error(err)
	}

	index, err = repo.LoadIndexFile(destIndex)
	if err != nil {
		t.Fatal(err)
	}

	// verify it didn't create an empty index.yaml or empty index.json
	// and the merged happened
	if len(index.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entries, got %d: %#v",
			expectedNumberOfEntries, len(index.Entries), index.Entries)
	}

	vs = index.Entries["compressedchart"]
	if len(vs) != expectedNumberOfVersions {
		t.Errorf("expected %d versions, got %d: %#v",
			expectedNumberOfVersions, len(vs), vs)
	}

	expectedVersion = "0.3.0"
	if vs[0].Version != expectedVersion {
		t.Errorf("expected %q, got %q", expectedVersion, vs[0].Version)
	}

	jsonIndex, err = repo.LoadIndexJSONFile(destJSONIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(jsonIndex.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entry in json index, got %d: %#v",
			expectedNumberOfEntries, len(jsonIndex.Entries), jsonIndex.Entries)
	}

	versionsInJSONIndex = jsonIndex.Entries["compressedchart"]
	if len(versionsInJSONIndex) != expectedNumberOfVersions {
		t.Errorf("expected %d versions in json index, got %d: %#v",
			expectedNumberOfVersions, len(versionsInJSONIndex), versionsInJSONIndex)
	}

	expectedVersionInJSONIndex = "0.3.0"
	if versionsInJSONIndex[0].Version != expectedVersionInJSONIndex {
		t.Errorf("expected %q in json index, got %q",
			expectedVersionInJSONIndex, versionsInJSONIndex[0].Version)
	}

	// test that index.yaml and index.json gets generated on
	// merge even when given index.json file doesn't doesn't exist
	if err := os.Remove(destIndex); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(destJSONIndex); err != nil {
		t.Fatal(err)
	}

	buf = bytes.NewBuffer(nil)
	c = newRepoIndexCmd(buf)
	expectedNumberOfEntries = 2
	expectedNumberOfVersions = 1

	if err = c.ParseFlags([]string{"--merge-json-index", destJSONIndex}); err != nil {
		t.Error(err)
	}
	if err := c.RunE(c, []string{dir}); err != nil {
		t.Error(err)
	}

	index, err = repo.LoadIndexFile(destIndex)
	if err != nil {
		t.Fatal(err)
	}

	// verify it didn't create an empty index.yaml or empty index.json
	// and the merged happened
	if len(index.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entries, got %d: %#v",
			expectedNumberOfEntries, len(index.Entries), index.Entries)
	}

	vs = index.Entries["compressedchart"]
	if len(vs) != expectedNumberOfVersions {
		t.Errorf("expected %d versions, got %d: %#v",
			expectedNumberOfVersions, len(vs), vs)
	}

	expectedVersion = "0.3.0"
	if vs[0].Version != expectedVersion {
		t.Errorf("expected %q, got %q", expectedVersion, vs[0].Version)
	}

	jsonIndex, err = repo.LoadIndexJSONFile(destJSONIndex)
	if err != nil {
		t.Fatal(err)
	}

	if len(jsonIndex.Entries) != expectedNumberOfEntries {
		t.Errorf("expected %d entry in json index, got %d: %#v",
			expectedNumberOfEntries, len(jsonIndex.Entries), jsonIndex.Entries)
	}

	versionsInJSONIndex = jsonIndex.Entries["compressedchart"]
	if len(versionsInJSONIndex) != expectedNumberOfVersions {
		t.Errorf("expected %d versions in json index, got %d: %#v",
			expectedNumberOfVersions, len(versionsInJSONIndex), versionsInJSONIndex)
	}

	expectedVersionInJSONIndex = "0.3.0"
	if versionsInJSONIndex[0].Version != expectedVersionInJSONIndex {
		t.Errorf("expected %q in json index, got %q",
			expectedVersionInJSONIndex, versionsInJSONIndex[0].Version)
	}
	// })
}

func TestRepoIndexCmd_Another(t *testing.T) {
	t.Run("passing both --merge and --merge-json-index should fail", func(t *testing.T) {
		dir := ensure.TempDir(t)
		buf := bytes.NewBuffer(nil)
		c := newRepoIndexCmd(buf)

		destIndex := filepath.Join(dir, "index.yaml")
		destJSONIndex := filepath.Join(dir, "index.json")

		if err := c.ParseFlags([]string{"--merge", destIndex,
			"--merge-json-index", destJSONIndex}); err != nil {
			t.Error(err)
		}
		err := c.RunE(c, []string{dir})
		if err == nil {
			t.Error("expected error but got nil")
			return
		}

		expectedError := "only one of --merge and --merge-json-index can be passed"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("expected error '%s' but got '%s'", expectedError, err.Error())
		}
	})
}

func linkOrCopy(old, new string) error {
	if err := os.Link(old, new); err != nil {
		return copyFile(old, new)
	}

	return nil
}

func copyFile(src, dest string) error {
	i, err := os.Open(src)
	if err != nil {
		return err
	}
	defer i.Close()

	o, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer o.Close()

	_, err = io.Copy(o, i)

	return err
}
