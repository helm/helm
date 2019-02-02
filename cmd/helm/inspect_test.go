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
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"k8s.io/helm/pkg/repo/repotest"
)

func TestInspect(t *testing.T) {
	b := bytes.NewBuffer(nil)

	insp := &inspectCmd{
		chartpath: "testdata/testcharts/alpine",
		output:    all,
		out:       b,
	}
	insp.run()

	// Load the data from the textfixture directly.
	cdata, err := ioutil.ReadFile("testdata/testcharts/alpine/Chart.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadFile("testdata/testcharts/alpine/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	readmeData, err := ioutil.ReadFile("testdata/testcharts/alpine/README.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(b.String(), "---", 3)
	if len(parts) != 3 {
		t.Fatalf("Expected 2 parts, got %d", len(parts))
	}

	expect := []string{
		strings.Replace(strings.TrimSpace(string(cdata)), "\r", "", -1),
		strings.Replace(strings.TrimSpace(string(data)), "\r", "", -1),
		strings.Replace(strings.TrimSpace(string(readmeData)), "\r", "", -1),
	}

	// Problem: ghodss/yaml doesn't marshal into struct order. To solve, we
	// have to carefully craft the Chart.yaml to match.
	for i, got := range parts {
		got = strings.Replace(strings.TrimSpace(got), "\r", "", -1)
		if got != expect[i] {
			t.Errorf("Expected\n%q\nGot\n%q\n", expect[i], got)
		}
	}

	// Regression tests for missing values. See issue #1024.
	b.Reset()
	insp = &inspectCmd{
		chartpath: "testdata/testcharts/novals",
		output:    "values",
		out:       b,
	}
	insp.run()
	if b.Len() != 0 {
		t.Errorf("expected empty values buffer, got %q", b.String())
	}
}

func TestInspectPreReleaseChart(t *testing.T) {
	hh, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := resetEnv()
	defer func() {
		os.RemoveAll(hh.String())
		cleanup()
	}()

	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()

	if _, err := srv.CopyCharts("testdata/testcharts/*.tgz*"); err != nil {
		t.Fatal(err)
	}
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		args        []string
		flags       []string
		fail        bool
		expectedErr string
	}{
		{
			name:        "inspect pre-release chart",
			args:        []string{"prerelease"},
			fail:        true,
			expectedErr: "chart \"prerelease\" not found",
		},
		{
			name:  "inspect pre-release chart with 'devel' flag",
			args:  []string{"prerelease"},
			flags: []string{"--devel"},
			fail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.flags = append(tt.flags, "--repo", srv.URL())
			cmd := newInspectCmd(ioutil.Discard)
			cmd.SetArgs(tt.args)
			cmd.ParseFlags(tt.flags)
			if err := cmd.RunE(cmd, tt.args); err != nil {
				if tt.fail {
					if !strings.Contains(err.Error(), tt.expectedErr) {
						t.Errorf("%q expected error: %s, got: %s", tt.name, tt.expectedErr, err.Error())
					}
					return
				}
				t.Errorf("%q reported error: %s", tt.name, err)
			}
		})
	}
}
