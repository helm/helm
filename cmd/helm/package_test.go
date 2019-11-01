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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestPackage(t *testing.T) {
	statFileMsg := "no such file or directory"
	if runtime.GOOS == "windows" {
		statFileMsg = "The system cannot find the file specified."
	}

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
			name:   "package --values does-not-exist",
			args:   []string{"testdata/testcharts/alpine"},
			flags:  map[string]string{"values": "does-not-exist"},
			expect: fmt.Sprintf("does-not-exist: %s", statFileMsg),
			err:    true,
		},
		{
			name: "package testdata/testcharts/chart-bad-type",
			args: []string{"testdata/testcharts/chart-bad-type"},
			err:  true,
		},
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cachePath := ensure.TempDir(t)
			defer testChdir(t, cachePath)()

			if err := os.MkdirAll("toot", 0777); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			c := newPackageCmd(&buf)

			// This is an unfortunate byproduct of the tmpdir
			if v, ok := tt.flags["keyring"]; ok && len(v) > 0 {
				tt.flags["keyring"] = filepath.Join(origDir, v)
			}

			setFlags(c, tt.flags)
			re := regexp.MustCompile(tt.expect)

			adjustedArgs := make([]string, len(tt.args))
			for i, f := range tt.args {
				adjustedArgs[i] = filepath.Join(origDir, f)
			}

			err := c.RunE(c, adjustedArgs)
			if err != nil {
				if tt.err && re.MatchString(err.Error()) {
					return
				}
				t.Fatalf("%q: expected error %q, got %q", tt.name, tt.expect, err)
			}

			if !re.Match(buf.Bytes()) {
				t.Errorf("%q: expected output %q, got %q", tt.name, tt.expect, buf.String())
			}

			if len(tt.hasfile) > 0 {
				if fi, err := os.Stat(tt.hasfile); err != nil {
					t.Errorf("%q: expected file %q, got err %q", tt.name, tt.hasfile, err)
				} else if fi.Size() == 0 {
					t.Errorf("%q: file %q has zero bytes.", tt.name, tt.hasfile)
				}
			}

			if v, ok := tt.flags["sign"]; ok && v == "1" {
				if fi, err := os.Stat(tt.hasfile + ".prov"); err != nil {
					t.Errorf("%q: expected provenance file", tt.name)
				} else if fi.Size() == 0 {
					t.Errorf("%q: provenance file is empty", tt.name)
				}
			}
		})
	}
}

func TestSetAppVersion(t *testing.T) {
	var ch *chart.Chart
	expectedAppVersion := "app-version-foo"

	dir := ensure.TempDir(t)

	c := newPackageCmd(&bytes.Buffer{})
	flags := map[string]string{
		"destination": dir,
		"app-version": expectedAppVersion,
	}
	setFlags(c, flags)
	if err := c.RunE(c, []string{"testdata/testcharts/alpine"}); err != nil {
		t.Errorf("unexpected error %q", err)
	}

	chartPath := filepath.Join(dir, "alpine-0.1.0.tgz")
	if fi, err := os.Stat(chartPath); err != nil {
		t.Errorf("expected file %q, got err %q", chartPath, err)
	} else if fi.Size() == 0 {
		t.Errorf("file %q has zero bytes.", chartPath)
	}
	ch, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("unexpected error loading packaged chart: %v", err)
	}
	if ch.Metadata.AppVersion != expectedAppVersion {
		t.Errorf("expected app-version %q, found %q", expectedAppVersion, ch.Metadata.AppVersion)
	}
}

func TestPackageValues(t *testing.T) {
	defer resetEnv()()

	repoFile := "testdata/helmhome/helm/repositories.yaml"

	testCases := []struct {
		desc               string
		args               []string
		valuefilesContents []string
		flags              map[string]string
		expected           []string
	}{
		{
			desc:               "helm package, single values file",
			args:               []string{"testdata/testcharts/alpine"},
			flags:              map[string]string{"repository-config": repoFile},
			valuefilesContents: []string{"Name: chart-name-foo"},
			expected:           []string{"Name: chart-name-foo"},
		},
		{
			desc:               "helm package, multiple values files",
			args:               []string{"testdata/testcharts/alpine"},
			flags:              map[string]string{"repository-config": repoFile},
			valuefilesContents: []string{"Name: chart-name-foo", "foo: bar"},
			expected:           []string{"Name: chart-name-foo", "foo: bar"},
		},
		{
			desc:     "helm package, with set option",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    map[string]string{"set": "Name=chart-name-foo", "repository-config": repoFile},
			expected: []string{"Name: chart-name-foo"},
		},
		{
			desc:               "helm package, set takes precedence over value file",
			args:               []string{"testdata/testcharts/alpine"},
			valuefilesContents: []string{"Name: chart-name-foo"},
			flags:              map[string]string{"set": "Name=chart-name-bar", "repository-config": repoFile},
			expected:           []string{"Name: chart-name-bar"},
		},
	}

	for _, tc := range testCases {
		var files []string
		for _, contents := range tc.valuefilesContents {
			f := createValuesFile(t, contents)
			files = append(files, f)
		}
		valueFiles := strings.Join(files, ",")

		expected, err := chartutil.ReadValues([]byte(strings.Join(tc.expected, "\n")))
		if err != nil {
			t.Errorf("unexpected error parsing values: %q", err)
		}

		outputDir := ensure.TempDir(t)

		if len(tc.flags) == 0 {
			tc.flags = make(map[string]string)
		}
		tc.flags["destination"] = outputDir

		if len(valueFiles) > 0 {
			tc.flags["values"] = valueFiles
		}

		cmd := newPackageCmd(&bytes.Buffer{})
		setFlags(cmd, tc.flags)
		if err := cmd.RunE(cmd, tc.args); err != nil {
			t.Fatalf("unexpected error: %q", err)
		}

		outputFile := filepath.Join(outputDir, "alpine-0.1.0.tgz")
		verifyOutputChartExists(t, outputFile)

		actual, err := getChartValues(outputFile)
		if err != nil {
			t.Fatalf("unexpected error extracting chart values: %q", err)
		}

		verifyValues(t, actual, expected)
	}
}

func createValuesFile(t *testing.T, data string) string {
	outputDir := ensure.TempDir(t)

	outputFile := filepath.Join(outputDir, "values.yaml")
	if err := ioutil.WriteFile(outputFile, []byte(data), 0644); err != nil {
		t.Fatalf("err: %s", err)
	}
	return outputFile
}

func getChartValues(chartPath string) (chartutil.Values, error) {
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, err
	}
	return chart.Values, nil
}

func verifyValues(t *testing.T, actual, expected map[string]interface{}) {
	t.Helper()
	for key, value := range expected {
		if got := actual[key]; got != value {
			t.Errorf("Expected %q, got %q (%v)", value, got, actual)
		}
	}
}

func verifyOutputChartExists(t *testing.T, chartPath string) {
	if chartFile, err := os.Stat(chartPath); err != nil {
		t.Errorf("expected file %q, got err %q", chartPath, err)
	} else if chartFile.Size() == 0 {
		t.Errorf("file %q has zero bytes.", chartPath)
	}
}

func setFlags(cmd *cobra.Command, flags map[string]string) {
	dest := cmd.Flags()
	for f, v := range flags {
		dest.Set(f, v)
	}
}
