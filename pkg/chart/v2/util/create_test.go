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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

func TestCreate(t *testing.T) {
	tdir := t.TempDir()

	c, err := Create("foo", tdir)
	require.NoError(t, err)

	dir := filepath.Join(tdir, "foo")

	mychart, err := loader.LoadDir(c)
	require.NoError(t, err, "Failed to load newly created chart %q", c)
	assert.Equal(t, "foo", mychart.Name(), "Expected name to be 'foo', got %q", mychart.Name())

	for _, f := range []string{
		ChartfileName,
		DeploymentName,
		HelpersName,
		IgnorefileName,
		NotesName,
		ServiceAccountName,
		ServiceName,
		TemplatesDir,
		TemplatesTestsDir,
		TestConnectionName,
		ValuesfileName,
	} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.NoErrorf(t, err, "Expected %s file", f)
	}
}

func TestCreateFrom(t *testing.T) {
	tdir := t.TempDir()

	cf := &chart.Metadata{
		APIVersion: chart.APIVersionV1,
		Name:       "foo",
		Version:    "0.1.0",
	}
	srcdir := "./testdata/frobnitz/charts/mariner"

	require.NoError(t, CreateFrom(cf, tdir, srcdir))

	dir := filepath.Join(tdir, "foo")
	c := filepath.Join(tdir, cf.Name)
	mychart, err := loader.LoadDir(c)
	require.NoError(t, err, "Failed to load newly created chart %q", c)
	assert.Equal(t, "foo", mychart.Name(), "Expected name to be 'foo', got %q", mychart.Name())

	for _, f := range []string{
		ChartfileName,
		ValuesfileName,
		filepath.Join(TemplatesDir, "placeholder.tpl"),
	} {
		_, err := os.Stat(filepath.Join(dir, f))
		require.NoErrorf(t, err, "Expected %s file", f)

		// Check each file to make sure <CHARTNAME> has been replaced
		b, err := os.ReadFile(filepath.Join(dir, f))
		require.NoError(t, err, "Unable to read file %s", f)
		assert.Falsef(t, bytes.Contains(b, []byte("<CHARTNAME>")), "File %s contains <CHARTNAME>", f)
	}
}

// TestCreate_Overwrite is a regression test for making sure that files are overwritten.
func TestCreate_Overwrite(t *testing.T) {
	tdir := t.TempDir()

	var errlog bytes.Buffer

	_, err := Create("foo", tdir)
	require.NoError(t, err)

	dir := filepath.Join(tdir, "foo")

	tplname := filepath.Join(dir, "templates", "hpa.yaml")
	writeFile(tplname, []byte("FOO"))

	// Now re-run the create
	Stderr = &errlog
	_, err = Create("foo", tdir)
	require.NoError(t, err)

	data, err := os.ReadFile(tplname)
	require.NoError(t, err)
	require.NotEqual(t, "FOO", string(data), "File that should have been modified was not.")
	assert.NotEqual(t, 0, errlog.Len(), "Expected warnings about overwriting files.")
}

func TestValidateChartName(t *testing.T) {
	for name, shouldPass := range map[string]bool{
		"":                              false,
		"abcdefghijklmnopqrstuvwxyz-_.": true,
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ-_.": true,
		"$hello":                        false,
		"Hellô":                         false,
		"he%%o":                         false,
		"he\nllo":                       false,

		"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"abcdefghijklmnopqrstuvwxyz-_." +
			"ABCDEFGHIJKLMNOPQRSTUVWXYZ-_.": false,
	} {
		err := validateChartName(name)
		if shouldPass {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}
