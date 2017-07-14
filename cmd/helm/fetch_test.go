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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"k8s.io/helm/pkg/repo/repotest"
)

func TestFetchCmd(t *testing.T) {
	hh, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := resetEnv()
	defer func() {
		os.RemoveAll(hh.String())
		cleanup()
	}()
	srv := repotest.NewServer(hh.String())
	defer srv.Stop()

	settings.Home = hh

	// all flags will get "--home=TMDIR -d outdir" appended.
	tests := []struct {
		name         string
		chart        string
		flags        []string
		fail         bool
		failExpect   string
		expectFile   string
		expectDir    bool
		expectVerify bool
	}{
		{
			name:       "Basic chart fetch",
			chart:      "test/signtest",
			expectFile: "./signtest-0.1.0.tgz",
		},
		{
			name:       "Chart fetch with version",
			chart:      "test/signtest",
			flags:      []string{"--version", "0.1.0"},
			expectFile: "./signtest-0.1.0.tgz",
		},
		{
			name:       "Fail chart fetch with non-existent version",
			chart:      "test/signtest",
			flags:      []string{"--version", "99.1.0"},
			fail:       true,
			failExpect: "no such chart",
		},
		{
			name:       "Fail fetching non-existent chart",
			chart:      "test/nosuchthing",
			failExpect: "Failed to fetch",
			fail:       true,
		},
		{
			name:         "Fetch and verify",
			chart:        "test/signtest",
			flags:        []string{"--verify", "--keyring", "testdata/helm-test-key.pub"},
			expectFile:   "./signtest-0.1.0.tgz",
			expectVerify: true,
		},
		{
			name:       "Fetch and fail verify",
			chart:      "test/reqtest",
			flags:      []string{"--verify", "--keyring", "testdata/helm-test-key.pub"},
			failExpect: "Failed to fetch provenance",
			fail:       true,
		},
		{
			name:       "Fetch and untar",
			chart:      "test/signtest",
			flags:      []string{"--untar", "--untardir", "signtest"},
			expectFile: "./signtest",
			expectDir:  true,
		},
		{
			name:         "Fetch, verify, untar",
			chart:        "test/signtest",
			flags:        []string{"--verify", "--keyring", "testdata/helm-test-key.pub", "--untar", "--untardir", "signtest"},
			expectFile:   "./signtest",
			expectDir:    true,
			expectVerify: true,
		},
		{
			name:       "Chart fetch using repo URL",
			chart:      "signtest",
			expectFile: "./signtest-0.1.0.tgz",
			flags:      []string{"--repo", srv.URL()},
		},
		{
			name:       "Fail fetching non-existent chart on repo URL",
			chart:      "someChart",
			flags:      []string{"--repo", srv.URL()},
			failExpect: "Failed to fetch chart",
			fail:       true,
		},
		{
			name:       "Specific version chart fetch using repo URL",
			chart:      "signtest",
			expectFile: "./signtest-0.1.0.tgz",
			flags:      []string{"--repo", srv.URL(), "--version", "0.1.0"},
		},
		{
			name:       "Specific version chart fetch using repo URL",
			chart:      "signtest",
			flags:      []string{"--repo", srv.URL(), "--version", "0.2.0"},
			failExpect: "Failed to fetch chart version",
			fail:       true,
		},
	}

	if _, err := srv.CopyCharts("testdata/testcharts/*.tgz*"); err != nil {
		t.Fatal(err)
	}
	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		outdir := filepath.Join(hh.String(), "testout")
		os.RemoveAll(outdir)
		os.Mkdir(outdir, 0755)

		buf := bytes.NewBuffer(nil)
		cmd := newFetchCmd(buf)
		tt.flags = append(tt.flags, "-d", outdir)
		cmd.ParseFlags(tt.flags)
		if err := cmd.RunE(cmd, []string{tt.chart}); err != nil {
			if tt.fail {
				continue
			}
			t.Errorf("%q reported error: %s", tt.name, err)
			continue
		}
		if tt.expectVerify {
			pointerAddressPattern := "0[xX][A-Fa-f0-9]+"
			sha256Pattern := "[A-Fa-f0-9]{64}"
			verificationRegex := regexp.MustCompile(
				fmt.Sprintf("Verification: &{%s sha256:%s signtest-0.1.0.tgz}\n", pointerAddressPattern, sha256Pattern))
			if !verificationRegex.MatchString(buf.String()) {
				t.Errorf("%q: expected match for regex %s, got %s", tt.name, verificationRegex, buf.String())
			}
		}

		ef := filepath.Join(outdir, tt.expectFile)
		fi, err := os.Stat(ef)
		if err != nil {
			t.Errorf("%q: expected a file at %s. %s", tt.name, ef, err)
		}
		if fi.IsDir() != tt.expectDir {
			t.Errorf("%q: expected directory=%t, but it's not.", tt.name, tt.expectDir)
		}
	}
}
