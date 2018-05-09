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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	shellwords "github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

	"k8s.io/helm/internal/test"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

func executeCommand(c helm.Interface, cmd string) (string, error) {
	_, output, err := executeCommandC(c, cmd)
	return output, err
}

func executeCommandC(client helm.Interface, cmd string) (*cobra.Command, string, error) {
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return nil, "", err
	}
	buf := new(bytes.Buffer)
	root := newRootCmd(client, buf, args)
	root.SetOutput(buf)
	root.SetArgs(args)

	c, err := root.ExecuteC()

	return c, buf.String(), err
}

func testReleaseCmd(t *testing.T, tests []releaseCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

// releaseCase describes a test case that works with releases.
type releaseCase struct {
	name      string
	cmd       string
	golden    string
	wantError bool
	// Rels are the available releases at the start of the test.
	rels          []*release.Release
	testRunStatus map[string]release.TestRunStatus
}

// tempHelmHome sets up a Helm Home in a temp dir.
//
// This does not clean up the directory. You must do that yourself.
// You  must also set helmHome yourself.
func tempHelmHome(t *testing.T) (helmpath.Home, error) {
	oldhome := settings.Home
	dir, err := ioutil.TempDir("", "helm_home-")
	if err != nil {
		return helmpath.Home("n/"), err
	}

	settings.Home = helmpath.Home(dir)
	if err := ensureTestHome(t, settings.Home); err != nil {
		return helmpath.Home("n/"), err
	}
	settings.Home = oldhome
	return helmpath.Home(dir), nil
}

// ensureTestHome creates a home directory like ensureHome, but without remote references.
//
// t is used only for logging.
func ensureTestHome(t *testing.T, home helmpath.Home) error {
	t.Helper()
	for _, p := range []string{
		home.String(),
		home.Repository(),
		home.Cache(),
		home.Plugins(),
		home.Starters(),
	} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return fmt.Errorf("Could not create %s: %s", p, err)
		}
	}

	repoFile := home.RepositoryFile()
	if _, err := os.Stat(repoFile); err != nil {
		rf := repo.NewRepoFile()
		rf.Add(&repo.Entry{
			Name:  "charts",
			URL:   "http://example.com/foo",
			Cache: "charts-index.yaml",
		})
		if err := rf.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	}
	if r, err := repo.LoadRepositoriesFile(repoFile); err == repo.ErrRepoOutOfDate {
		t.Log("Updating repository file format...")
		if err := r.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	}

	t.Logf("$HELM_HOME has been configured at %s.\n", home)
	return nil

}

func TestRootCmd(t *testing.T) {
	cleanup := resetEnv()
	defer cleanup()

	tests := []struct {
		name, args, home string
		envars           map[string]string
	}{
		{
			name: "defaults",
			args: "home",
			home: filepath.Join(os.Getenv("HOME"), "/.helm"),
		},
		{
			name: "with --home set",
			args: "--home /foo",
			home: "/foo",
		},
		{
			name: "subcommands with --home set",
			args: "home --home /foo",
			home: "/foo",
		},
		{
			name:   "with $HELM_HOME set",
			args:   "home",
			envars: map[string]string{"HELM_HOME": "/bar"},
			home:   "/bar",
		},
		{
			name:   "subcommands with $HELM_HOME set",
			args:   "home",
			envars: map[string]string{"HELM_HOME": "/bar"},
			home:   "/bar",
		},
		{
			name:   "with $HELM_HOME and --home set",
			args:   "home --home /foo",
			envars: map[string]string{"HELM_HOME": "/bar"},
			home:   "/foo",
		},
	}

	// ensure not set locally
	os.Unsetenv("HELM_HOME")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Unsetenv("HELM_HOME")

			for k, v := range tt.envars {
				os.Setenv(k, v)
			}

			cmd, _, err := executeCommandC(nil, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if settings.Home.String() != tt.home {
				t.Errorf("expected home %q, got %q", tt.home, settings.Home)
			}
			homeFlag := cmd.Flag("home").Value.String()
			homeFlag = os.ExpandEnv(homeFlag)
			if homeFlag != tt.home {
				t.Errorf("expected home %q, got %q", tt.home, homeFlag)
			}
		})
	}
}

func resetEnv() func() {
	origSettings := settings
	origEnv := os.Environ()
	return func() {
		settings = origSettings
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}
