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
	"time"

	shellwords "github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

	"helm.sh/helm/internal/test"
	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/kube"
	"helm.sh/helm/pkg/release"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/storage"
	"helm.sh/helm/pkg/storage/driver"
)

func testTimestamper() time.Time { return time.Unix(242085845, 0).UTC() }

func init() {
	action.Timestamper = testTimestamper
}

func TestMain(m *testing.M) {
	os.Unsetenv("HELM_HOME")
	exitCode := m.Run()
	os.Exit(exitCode)
}

func testTempDir(t *testing.T) string {
	t.Helper()
	d, err := ioutil.TempDir("", "helm")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func runTestCmd(t *testing.T, tests []cmdTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			storage := storageFixture()
			for _, rel := range tt.rels {
				if err := storage.Create(rel); err != nil {
					t.Fatal(err)
				}
			}
			_, out, err := executeActionCommandC(storage, tt.cmd)
			if (err != nil) != tt.wantError {
				t.Errorf("expected error, got '%v'", err)
			}
			if tt.golden != "" {
				test.AssertGoldenString(t, out, tt.golden)
			}
		})
	}
}

func runTestActionCmd(t *testing.T, tests []cmdTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			store := storageFixture()
			for _, rel := range tt.rels {
				store.Create(rel)
			}
			_, out, err := executeActionCommandC(store, tt.cmd)
			if (err != nil) != tt.wantError {
				t.Errorf("expected error, got '%v'", err)
			}
			if tt.golden != "" {
				test.AssertGoldenString(t, out, tt.golden)
			}
		})
	}
}

func storageFixture() *storage.Storage {
	return storage.Init(driver.NewMemory())
}

func executeActionCommandC(store *storage.Storage, cmd string) (*cobra.Command, string, error) {
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return nil, "", err
	}
	buf := new(bytes.Buffer)

	actionConfig := &action.Configuration{
		Releases:     store,
		KubeClient:   &kube.PrintingKubeClient{Out: ioutil.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}

	root := newRootCmd(actionConfig, buf, args)
	root.SetOutput(buf)
	root.SetArgs(args)

	c, err := root.ExecuteC()

	return c, buf.String(), err
}

// cmdTestCase describes a test case that works with releases.
type cmdTestCase struct {
	name      string
	cmd       string
	golden    string
	wantError bool
	// Rels are the available releases at the start of the test.
	rels []*release.Release
}

func executeActionCommand(cmd string) (*cobra.Command, string, error) {
	return executeActionCommandC(storageFixture(), cmd)
}

// ensureTestHome creates a home directory like ensureHome, but without remote references.
func ensureTestHome(t *testing.T, home helmpath.Home) {
	t.Helper()
	for _, p := range []string{
		home.String(),
		home.Repository(),
		home.Cache(),
		home.Plugins(),
		home.Starters(),
	} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}

	repoFile := home.RepositoryFile()
	if _, err := os.Stat(repoFile); err != nil {
		rf := repo.NewFile()
		rf.Add(&repo.Entry{
			Name:  "charts",
			URL:   "http://example.com/foo",
			Cache: "charts-index.yaml",
		})
		if err := rf.WriteFile(repoFile, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if r, err := repo.LoadFile(repoFile); err == repo.ErrRepoOutOfDate {
		if err := r.WriteFile(repoFile, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// testHelmHome sets up a Helm Home in a temp dir.
func testHelmHome(t *testing.T) helmpath.Home {
	t.Helper()
	dir := helmpath.Home(testTempDir(t))
	ensureTestHome(t, dir)
	return dir
}

func resetEnv() func() {
	origSettings, origEnv := settings, os.Environ()
	return func() {
		os.Clearenv()
		settings = origSettings
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}

func testChdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { os.Chdir(old) }
}
