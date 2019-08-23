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
	"strings"
	"testing"

	"helm.sh/helm/pkg/provenance"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/repo/repotest"
)

func TestDependencyBuildCmd(t *testing.T) {
	srv, err := repotest.NewTempServer("testdata/testcharts/*.tgz")
	defer srv.Stop()
	if err != nil {
		t.Fatal(err)
	}

	rootDir := srv.Root()

	chartname := "depbuild"
	createTestingChart(t, rootDir, chartname, srv.URL())
	repoFile := filepath.Join(srv.Root(), "repositories.yaml")

	cmd := fmt.Sprintf("dependency build '%s' --repository-config %s", filepath.Join(rootDir, chartname), repoFile)
	_, out, err := executeActionCommand(cmd)

	// In the first pass, we basically want the same results as an update.
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatalf("%+v", err)
	}

	if !strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", out)
	}

	// Make sure the actual file got downloaded.
	expect := filepath.Join(srv.Root(), chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	// In the second pass, we want to remove the chart's request dependency,
	// then see if it restores from the lock.
	lockfile := filepath.Join(srv.Root(), chartname, "Chart.lock")
	if _, err := os.Stat(lockfile); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(expect); err != nil {
		t.Fatal(err)
	}

	_, out, err = executeActionCommand(cmd)
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// Now repeat the test that the dependency exists.
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	// Make sure that build is also fetching the correct version.
	hash, err := provenance.DigestFile(expect)
	if err != nil {
		t.Fatal(err)
	}

	i, err := repo.LoadIndexFile(filepath.Join(srv.Root(), "index.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	reqver := i.Entries["reqtest"][0]
	if h := reqver.Digest; h != hash {
		t.Errorf("Failed hash match: expected %s, got %s", hash, h)
	}
	if v := reqver.Version; v != "0.1.0" {
		t.Errorf("mismatched versions. Expected %q, got %q", "0.1.0", v)
	}
}
