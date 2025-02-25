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
	"path/filepath"
	"testing"

	"io/fs"

	"helm.sh/helm/v3/pkg/repo/repotest"
	"sigs.k8s.io/yaml"
)

func TestRepoImportCmd(t *testing.T) {
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// A second test server is setup to verify URL changing
	srv2, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv2.Stop()

	tmpdir := filepath.Join(t.TempDir(), "path-component.yaml/data")
	err = os.MkdirAll(tmpdir, 0777)
	if err != nil {
		t.Fatal(err)
	}
	repoFile := filepath.Join(tmpdir, "repositories.yaml")
	repoImportFile := filepath.Join(tmpdir, "repositories-import.yaml")

	var repositories = []repositoryElement{
		{Name: "repo1", URL: srv.URL()},
		{Name: "repo2", URL: srv2.URL()},
	}

	data, err := yaml.Marshal(&repositories)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(repoImportFile, data, fs.FileMode(0644))
	if err != nil {
		t.Fatal(err)
	}

	tests := []cmdTestCase{
		{
			name:   "import repositories",
			cmd:    fmt.Sprintf("repo import %s --repository-config %s --repository-cache %s", repoImportFile, repoFile, tmpdir),
			golden: "output/repo-import.txt",
		},
		{
			name:   "import repositories second time",
			cmd:    fmt.Sprintf("repo import %s --repository-config %s --repository-cache %s", repoImportFile, repoFile, tmpdir),
			golden: "output/repo-import2.txt",
		},
	}

	runTestCmd(t, tests)
}

func TestRepoImportFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo import", true)
	checkFileCompletion(t, "repo import file", false)
}

func TestRepoImportOnNonExistingFile(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()
	tmpdir := filepath.Join(t.TempDir(), "path-component.yaml/data")
	err = os.MkdirAll(tmpdir, 0777)
	if err != nil {
		t.Fatal(err)
	}
	repoFile := filepath.Join(tmpdir, "repositories.yaml")
	nonExistingFileName := "non-existing-file"

	o := &repoImportOptions{
		importedFilePath: nonExistingFileName,
		repoFile:         repoFile,
		repoCache:        tmpdir,
	}

	wantErrorMsg := fmt.Sprintf("open %s: no such file or directory", nonExistingFileName)

	if err := o.run(io.Discard); err != nil {
		if wantErrorMsg != err.Error() {
			t.Fatalf("Actual error %s, not equal to expected error %s", err, wantErrorMsg)
		}
	} else {
		t.Fatalf("expect reported an error.")
	}
}

func TestRepoImportOnNonYamlFile(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()
	tmpdir := filepath.Join(t.TempDir(), "path-component.yaml/data")
	err = os.MkdirAll(tmpdir, 0777)
	if err != nil {
		t.Fatal(err)
	}
	repoFile := filepath.Join(tmpdir, "repositories.yaml")
	repoImportFile := filepath.Join(tmpdir, "repositories-import.txt")
	err = os.WriteFile(repoImportFile, []byte("This is not a yaml file"), fs.FileMode(0644))
	if err != nil {
		t.Fatal(err)
	}

	o := &repoImportOptions{
		importedFilePath: repoImportFile,
		repoFile:         repoFile,
		repoCache:        tmpdir,
	}

	wantErrorMsg := fmt.Sprintf("%s is an invalid YAML file", repoImportFile)

	if err := o.run(io.Discard); err != nil {
		if wantErrorMsg != err.Error() {
			t.Fatalf("Actual error %s, not equal to expected error %s", err, wantErrorMsg)
		}
	} else {
		t.Fatalf("expect reported an error.")
	}
}

func TestRepoImportOnYamlFileWithInvalidStructure(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()
	tmpdir := filepath.Join(t.TempDir(), "path-component.yaml/data")
	err = os.MkdirAll(tmpdir, 0777)
	if err != nil {
		t.Fatal(err)
	}
	repoFile := filepath.Join(tmpdir, "repositories.yaml")
	repoImportFile := filepath.Join(tmpdir, "repositories-import.txt")
	err = os.WriteFile(repoImportFile, []byte("- firstKey: firstValue\n  secondKey: secondValue"), fs.FileMode(0644))
	if err != nil {
		t.Fatal(err)
	}

	o := &repoImportOptions{
		importedFilePath: repoImportFile,
		repoFile:         repoFile,
		repoCache:        tmpdir,
	}

	wantErrorMsg := fmt.Sprintf("%s is an invalid YAML file", repoImportFile)

	if err := o.run(io.Discard); err != nil {
		if wantErrorMsg != err.Error() {
			t.Fatalf("Actual error %s, not equal to expected error %s", err, wantErrorMsg)
		}
	} else {
		t.Fatalf("expect reported an error.")
	}
}
