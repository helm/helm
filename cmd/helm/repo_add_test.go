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
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

var testName = "test-name"

func TestRepoAddCmd(t *testing.T) {
	srv, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := resetEnv()
	defer func() {
		srv.Stop()
		os.RemoveAll(thome.String())
		cleanup()
	}()
	if err := ensureTestHome(thome, t); err != nil {
		t.Fatal(err)
	}

	settings.Home = thome

	tests := []releaseCase{
		{
			name:     "add a repository",
			args:     []string{testName, srv.URL()},
			expected: "\"" + testName + "\" has been added to your repositories",
		},
	}

	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newRepoAddCmd(out)
	})
}

func TestRepoAdd(t *testing.T) {
	ts, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := resetEnv()
	hh := thome
	defer func() {
		ts.Stop()
		os.RemoveAll(thome.String())
		cleanup()
	}()
	if err := ensureTestHome(hh, t); err != nil {
		t.Fatal(err)
	}

	settings.Home = thome

	if err := addRepository(testName, ts.URL(), "", "", hh, "", "", "", true); err != nil {
		t.Error(err)
	}

	f, err := repo.LoadRepositoriesFile(hh.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	if !f.Has(testName) {
		t.Errorf("%s was not successfully inserted into %s", testName, hh.RepositoryFile())
	}

	if err := addRepository(testName, ts.URL(), "", "", hh, "", "", "", false); err != nil {
		t.Errorf("Repository was not updated: %s", err)
	}

	if err := addRepository(testName, ts.URL(), "", "", hh, "", "", "", false); err != nil {
		t.Errorf("Duplicate repository name was added")
	}
}
func TestRepoAddConcurrentGoRoutines(t *testing.T) {
	ts, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := resetEnv()
	defer func() {
		ts.Stop()
		os.RemoveAll(thome.String())
		cleanup()
	}()

	settings.Home = thome
	if err := ensureTestHome(settings.Home, t); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func(name string) {
			defer wg.Done()
			if err := addRepository(name, ts.URL(), "", "", settings.Home, "", "", "", true); err != nil {
				t.Error(err)
			}
		}(fmt.Sprintf("%s-%d", testName, i))
	}
	wg.Wait()

	f, err := repo.LoadRepositoriesFile(settings.Home.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	var name string
	for i := 0; i < 3; i++ {
		name = fmt.Sprintf("%s-%d", testName, i)
		if !f.Has(name) {
			t.Errorf("%s was not successfully inserted into %s", name, settings.Home.RepositoryFile())
		}
	}
}

// Same as TestRepoAddConcurrentGoRoutines but with repository additions in sub-processes
func TestRepoAddConcurrentSubProcesses(t *testing.T) {
	goWantHelperProcess := os.Getenv("GO_WANT_HELPER_PROCESS")
	if goWantHelperProcess == "" {
		// parent

		ts, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
		if err != nil {
			t.Fatal(err)
		}

		settings.Home = thome

		cleanup := resetEnv()
		defer func() {
			ts.Stop()
			os.RemoveAll(thome.String())
			cleanup()
		}()
		if err := ensureTestHome(settings.Home, t); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func(name string) {
				defer wg.Done()

				cmd := exec.Command(os.Args[0], "-test.run=^TestRepoAddConcurrentSubProcesses$")
				cmd.Env = append(os.Environ(), fmt.Sprintf("GO_WANT_HELPER_PROCESS=%s,%s", name, ts.URL()), fmt.Sprintf("HELM_HOME=%s", settings.Home))
				out, err := cmd.CombinedOutput()
				if len(out) > 0 || err != nil {
					t.Fatalf("child process: %q, %v", out, err)
				}
			}(fmt.Sprintf("%s-%d", testName, i))
		}
		wg.Wait()

		f, err := repo.LoadRepositoriesFile(settings.Home.RepositoryFile())
		if err != nil {
			t.Error(err)
		}

		var name string
		for i := 0; i < 2; i++ {
			name = fmt.Sprintf("%s-%d", testName, i)
			if !f.Has(name) {
				t.Errorf("%s was not successfully inserted into %s", name, settings.Home.RepositoryFile())
			}
		}
	} else {
		// child
		s := strings.Split(goWantHelperProcess, ",")
		settings.Home = helmpath.Home(os.Getenv("HELM_HOME"))
		repoName := s[0]
		tsURL := s[1]
		if err := addRepository(repoName, tsURL, "", "", settings.Home, "", "", "", true); err != nil {
			t.Fatal(err)
		}

		os.Exit(0)
	}
}
