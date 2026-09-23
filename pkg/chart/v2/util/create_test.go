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
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/engine"
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

func TestCreate_LabelValues(t *testing.T) {
	tdir := t.TempDir()

	c, err := Create("demo", tdir)
	require.NoError(t, err)

	mychart, err := loader.LoadDir(c)
	require.NoError(t, err)

	// "demo-" + version is 64 characters, so `trunc 63` cuts right after the
	// character that precedes the final "1".
	for name, tc := range map[string]struct {
		version    string
		appVersion string
	}{
		"truncated chart label ends in dot": {
			version:    "1.0.0-abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxy.1",
			appVersion: "1.16.0",
		},
		"truncated chart label ends in underscore": {
			version:    "1.0.0-abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxy+1",
			appVersion: "1.16.0",
		},
		"app version with build metadata longer than 63 characters": {
			version:    "0.1.0",
			appVersion: "v1.2.3+build.123456789012345678901234567890123456789012345678901234567890",
		},
	} {
		t.Run(name, func(t *testing.T) {
			mychart.Metadata.Version = tc.version
			mychart.Metadata.AppVersion = tc.appVersion

			opts := common.ReleaseOptions{Name: "demo", Namespace: "default", Revision: 1, IsInstall: true}
			vals, err := util.ToRenderValues(mychart, map[string]any{}, opts, common.DefaultCapabilities)
			require.NoError(t, err)

			out, err := new(engine.Engine).RenderWithContext(t.Context(), mychart, vals)
			require.NoError(t, err)

			var sa struct {
				Metadata struct {
					Labels map[string]string `json:"labels"`
				} `json:"metadata"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(out["demo/templates/serviceaccount.yaml"]), &sa))

			for _, key := range []string{"helm.sh/chart", "app.kubernetes.io/version"} {
				v, ok := sa.Metadata.Labels[key]
				require.True(t, ok, "expected label %q to be set", key)
				assert.Empty(t, validation.IsValidLabelValue(v), "label %s=%q", key, v)
			}
		})
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
