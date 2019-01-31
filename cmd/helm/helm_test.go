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

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/helm/pkg/tiller/environment"

	shellwords "github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

	"k8s.io/helm/internal/test"
	"k8s.io/helm/pkg/action"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
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

			c := &helm.FakeClient{
				Rels:          tt.rels,
				TestRunStatus: tt.testRunStatus,
			}
			out, err := executeCommand(c, tt.cmd)
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
		Releases:   store,
		KubeClient: &environment.PrintingKubeClient{Out: ioutil.Discard},
		Discovery:  fake.NewSimpleClientset().Discovery(),
		Log:        func(format string, v ...interface{}) {},
	}

	root := newRootCmd(nil, actionConfig, buf, args)
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
	rels          []*release.Release
	testRunStatus map[string]release.TestRunStatus
}

// deprecated: Switch to executeActionCommandC
func executeCommand(c helm.Interface, cmd string) (string, error) {
	_, output, err := executeCommandC(c, cmd)
	return output, err
}

// deprecated: Switch to executeActionCommandC
func executeCommandC(client helm.Interface, cmd string) (*cobra.Command, string, error) {
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return nil, "", err
	}
	buf := new(bytes.Buffer)

	actionConfig := &action.Configuration{
		Releases: storage.Init(driver.NewMemory()),
	}

	root := newRootCmd(client, actionConfig, buf, args)
	root.SetOutput(buf)
	root.SetArgs(args)

	c, err := root.ExecuteC()

	return c, buf.String(), err
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
