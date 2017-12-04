/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestSetVersion(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "prow",
			Version: "0.0.1",
		},
	}
	expect := "1.2.3-beta.5"
	if err := setVersion(c, expect); err != nil {
		t.Fatal(err)
	}

	if c.Metadata.Version != expect {
		t.Errorf("Expected %q, got %q", expect, c.Metadata.Version)
	}

	if err := setVersion(c, "monkeyface"); err == nil {
		t.Error("Expected bogus version to return an error.")
	}
}

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
			name:    "package --destination toot",
			args:    []string{"testdata/testcharts/alpine"},
			flags:   map[string]string{"destination": "toot"},
			expect:  "",
			hasfile: "toot/alpine-0.1.0.tgz",
		},
		{
			name:   "package --destination does-not-exist",
			args:   []string{"testdata/testcharts/alpine"},
			flags:  map[string]string{"destination": "does-not-exist"},
			expect: "stat does-not-exist: no such file or directory",
			err:    true,
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
	}

	// Because these tests are destructive, we run them in a tempdir.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "helm-package-test-")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Running tests in %s", tmp)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir("toot", 0777); err != nil {
		t.Fatal(err)
	}

	ensureTestHome(helmpath.Home(tmp), t)
	cleanup := resetEnv()
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmp)
		cleanup()
	}()

	settings.Home = helmpath.Home(tmp)

	for _, tt := range tests {
		buf := bytes.NewBuffer(nil)
		c := newPackageCmd(buf)

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
				continue
			}
			t.Errorf("%q: expected error %q, got %q", tt.name, tt.expect, err)
			continue
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
	}
}

func TestSetAppVersion(t *testing.T) {
	var ch *chart.Chart
	expectedAppVersion := "app-version-foo"
	tmp, _ := ioutil.TempDir("", "helm-package-app-version-")
	defer os.RemoveAll(tmp)

	c := newPackageCmd(&bytes.Buffer{})
	flags := map[string]string{
		"destination": tmp,
		"app-version": expectedAppVersion,
	}
	setFlags(c, flags)
	err := c.RunE(c, []string{"testdata/testcharts/alpine"})
	if err != nil {
		t.Errorf("unexpected error %q", err)
	}

	chartPath := filepath.Join(tmp, "alpine-0.1.0.tgz")
	if fi, err := os.Stat(chartPath); err != nil {
		t.Errorf("expected file %q, got err %q", chartPath, err)
	} else if fi.Size() == 0 {
		t.Errorf("file %q has zero bytes.", chartPath)
	}
	ch, err = chartutil.Load(chartPath)
	if err != nil {
		t.Errorf("unexpected error loading packaged chart: %v", err)
	}
	if ch.Metadata.AppVersion != expectedAppVersion {
		t.Errorf("expected app-version %q, found %q", expectedAppVersion, ch.Metadata.AppVersion)
	}
}

func setFlags(cmd *cobra.Command, flags map[string]string) {
	dest := cmd.Flags()
	for f, v := range flags {
		dest.Set(f, v)
	}
}
