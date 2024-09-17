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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
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
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cachePath := t.TempDir()
			defer testChdir(t, cachePath)()

			if err := os.MkdirAll("toot", 0777); err != nil {
				t.Fatal(err)
			}

			// This is an unfortunate byproduct of the tmpdir
			if v, ok := tt.flags["keyring"]; ok && len(v) > 0 {
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
			if err != nil {
				if tt.err && re.MatchString(err.Error()) {
					return
				}
				t.Fatalf("%q: expected error %q, got %q", tt.name, tt.expect, err)
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
	chartToPackage := "testdata/testcharts/alpine"
	dir := t.TempDir()
	cmd := fmt.Sprintf("package %s --destination=%s --app-version=%s", chartToPackage, dir, expectedAppVersion)
	_, output, err := executeActionCommand(cmd)
	if err != nil {
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}
	chartPath := filepath.Join(dir, "alpine-0.1.0.tgz")
	if fi, err := os.Stat(chartPath); err != nil {
		t.Errorf("expected file %q, got err %q", chartPath, err)
	} else if fi.Size() == 0 {
		t.Errorf("file %q has zero bytes.", chartPath)
	}
	ch, err = loader.Load(chartPath)
	if err != nil {
		t.Fatalf("unexpected error loading packaged chart: %v", err)
	}
	if ch.Metadata.AppVersion != expectedAppVersion {
		t.Errorf("expected app-version %q, found %q", expectedAppVersion, ch.Metadata.AppVersion)
	}
}

func TestPackageFileCompletion(t *testing.T) {
	checkFileCompletion(t, "package", true)
	checkFileCompletion(t, "package mypath", true) // Multiple paths can be given
}
