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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

func TestPackage(t *testing.T) {
	tests := []struct {
		name    string
		flags   map[string]string
		args    []string
		expect  string
		hasfile string
		err     bool
	}{
		{
			name:   "package without chart path",
			args:   []string{},
			flags:  map[string]string{},
			expect: "need at least one argument, the path to the chart",
			err:    true,
		},
		{
			name:   "package --sign, no --key",
			args:   []string{"testdata/testcharts/alpine"},
			flags:  map[string]string{"sign": "1"},
			expect: "key is required for signing a package",
			err:    true,
		},
		{
			name:   "package --sign, no --keyring",
			args:   []string{"testdata/testcharts/alpine"},
			flags:  map[string]string{"sign": "1", "key": "nosuchkey", "keyring": ""},
			expect: "keyring is required for signing a package",
			err:    true,
		},
		{
			name:    "package testdata/testcharts/alpine, no save",
			args:    []string{"testdata/testcharts/alpine"},
			flags:   map[string]string{"save": "0"},
			expect:  "",
			hasfile: "alpine-0.1.0.tgz",
		},
		{
			name:    "package testdata/testcharts/alpine",
			args:    []string{"testdata/testcharts/alpine"},
			expect:  "",
			hasfile: "alpine-0.1.0.tgz",
		},
		{
			name:    "package testdata/testcharts/issue1979",
			args:    []string{"testdata/testcharts/issue1979"},
			expect:  "",
			hasfile: "alpine-0.1.0.tgz",
		},
		{
			name:    "package --destination toot",
			args:    []string{"testdata/testcharts/alpine"},
			flags:   map[string]string{"destination": "toot"},
			expect:  "",
			hasfile: "toot/alpine-0.1.0.tgz",
		},
		{
			name:    "package --sign --key=KEY --keyring=KEYRING testdata/testcharts/alpine",
			args:    []string{"testdata/testcharts/alpine"},
			flags:   map[string]string{"sign": "1", "keyring": "testdata/helm-test-key.secret", "key": "helm-test"},
			expect:  "",
			hasfile: "alpine-0.1.0.tgz",
		},
		{
			name:    "package testdata/testcharts/chart-missing-deps",
			args:    []string{"testdata/testcharts/chart-missing-deps"},
			hasfile: "chart-missing-deps-0.1.0.tgz",
			err:     true,
		},
		{
			name: "package testdata/testcharts/chart-bad-type",
			args: []string{"testdata/testcharts/chart-bad-type"},
			err:  true,
		},
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			ensure.HelmHome(t)

			require.NoError(t, os.MkdirAll("toot", 0o777))

			// This is an unfortunate byproduct of the tmpdir
			if v, ok := tt.flags["keyring"]; ok && v != "" {
				tt.flags["keyring"] = filepath.Join(origDir, v)
			}

			re := regexp.MustCompile(tt.expect)

			adjustedArgs := make([]string, len(tt.args))
			for i, f := range tt.args {
				adjustedArgs[i] = filepath.Join(origDir, f)
			}

			cmd := []string{"package"}
			if len(adjustedArgs) > 0 {
				cmd = append(cmd, adjustedArgs...)
			}
			for k, v := range tt.flags {
				if v != "0" {
					cmd = append(cmd, fmt.Sprintf("--%s=%s", k, v))
				}
			}
			_, _, err = executeActionCommand(strings.Join(cmd, " "))
			if tt.err {
				require.Error(t, err)
				require.True(t, re.MatchString(err.Error()))
			} else {
				require.NoError(t, err)
				if tt.hasfile != "" {
					fi, err := os.Stat(tt.hasfile)
					require.NoErrorf(t, err, "%q: expected file %q", tt.name, tt.hasfile)
					assert.NotEqualf(t, 0, fi.Size(), "%q: file %q has zero bytes.", tt.name, tt.hasfile)
				}

				if v, ok := tt.flags["sign"]; ok && v == "1" {
					fi, err := os.Stat(tt.hasfile + ".prov")
					require.NoErrorf(t, err, "%q: expected provenance file", tt.name)
					assert.NotEqualf(t, 0, fi.Size(), "%q: provenance file is empty", tt.name)
				}
			}
		})
	}
}

func TestSetAppVersion(t *testing.T) {
	var ch *chart.Chart
	expectedAppVersion := "app-version-foo"
	chartToPackage := "testdata/testcharts/alpine"
	dir := t.TempDir()
	cmd := fmt.Sprintf("package %s --destination=%s --app-version=%s", chartToPackage, dir, expectedAppVersion)
	_, output, err := executeActionCommand(cmd)
	if err != nil {
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}
	chartPath := filepath.Join(dir, "alpine-0.1.0.tgz")
	fi, err := os.Stat(chartPath)
	require.NoErrorf(t, err, "expected file %q", chartPath)
	assert.NotEqualf(t, 0, fi.Size(), "file %q has zero bytes.", chartPath)
	ch, err = loader.Load(chartPath)
	require.NoError(t, err, "unexpected error loading packaged chart")
	assert.Equal(t, expectedAppVersion, ch.Metadata.AppVersion, "expected app-version %q, found %q", expectedAppVersion, ch.Metadata.AppVersion)
}

func TestPackageFileCompletion(t *testing.T) {
	checkFileCompletion(t, "package", true)
	checkFileCompletion(t, "package mypath", true) // Multiple paths can be given
}
