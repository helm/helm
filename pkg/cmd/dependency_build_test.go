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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/repo/v1"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestDependencyBuildCmd(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz"),
	)
	defer srv.Stop()

	rootDir := srv.Root()
	srv.LinkIndices()

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	if err != nil {
		t.Fatal(err)
	}

	ociChartName := "oci-depending-chart"
	c := createTestingMetadataForOCI(ociChartName, ociSrv.RegistryURL)
	if _, err := chartutil.Save(c, ociSrv.Dir); err != nil {
		t.Fatal(err)
	}
	ociSrv.Run(t, repotest.WithDependingChart(c))

	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}

	chartname := "depbuild"
	createTestingChart(t, rootDir, chartname, srv.URL())
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	cmd := fmt.Sprintf("dependency build '%s' --repository-config %s --repository-cache %s --plain-http", filepath.Join(rootDir, chartname), repoFile, rootDir)
	_, out, err := executeActionCommand(cmd)

	// In the first pass, we basically want the same results as an update.
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	if !strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", out)
	}

	// Make sure the actual file got downloaded.
	expect := filepath.Join(rootDir, chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	// In the second pass, we want to remove the chart's request dependency,
	// then see if it restores from the lock.
	lockfile := filepath.Join(rootDir, chartname, "Chart.lock")
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

	i, err := repo.LoadIndexFile(filepath.Join(rootDir, "index.yaml"))
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

	skipRefreshCmd := fmt.Sprintf("dependency build '%s' --skip-refresh --repository-config %s --repository-cache %s --plain-http", filepath.Join(rootDir, chartname), repoFile, rootDir)
	_, out, err = executeActionCommand(skipRefreshCmd)

	// In this pass, we check --skip-refresh option becomes effective.
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	if strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo did get updated\n%s", out)
	}

	// OCI dependencies
	if err := chartutil.SaveDir(c, dir()); err != nil {
		t.Fatal(err)
	}
	cmd = fmt.Sprintf("dependency build '%s' --repository-config %s --repository-cache %s --registry-config %s/config.json --plain-http",
		dir(ociChartName),
		dir("repositories.yaml"),
		dir(),
		dir())
	_, out, err = executeActionCommand(cmd)
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
	expect = dir(ociChartName, "charts/oci-dependent-chart-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyBuildCmdWithHelmV2Hash(t *testing.T) {
	chartName := "testdata/testcharts/issue-7233"

	cmd := fmt.Sprintf("dependency build '%s'", chartName)
	_, out, err := executeActionCommand(cmd)

	// Want to make sure the build can verify Helm v2 hash
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
}

func TestDefaultKeyring(t *testing.T) {
	touch := func(t *testing.T, path string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{"legacy keyring only", []string{"pubring.gpg"}, "pubring.gpg"},
		{"keybox only", []string{"pubring.kbx"}, "pubring.kbx"},
		{"legacy keyring preferred over keybox", []string{"pubring.gpg", "pubring.kbx"}, "pubring.gpg"},
		{"neither present falls back to legacy path", nil, "pubring.gpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("GNUPGHOME", dir)
			for _, f := range tt.files {
				touch(t, filepath.Join(dir, f))
			}
			if got, want := defaultKeyring(), filepath.Join(dir, tt.want); got != want {
				t.Errorf("expected %q, got %q", want, got)
			}
		})
	}

	t.Run("no GNUPGHOME falls back to the home directory", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("GNUPGHOME", home) // register restoration before unsetting
		os.Unsetenv("GNUPGHOME")

		gnupgDir := filepath.Join(home, ".gnupg")
		if err := os.MkdirAll(gnupgDir, 0o700); err != nil {
			t.Fatal(err)
		}

		if got, want := defaultKeyring(), filepath.Join(gnupgDir, "pubring.gpg"); got != want {
			t.Errorf("expected %q, got %q", want, got)
		}

		touch(t, filepath.Join(gnupgDir, "pubring.kbx"))
		if got, want := defaultKeyring(), filepath.Join(gnupgDir, "pubring.kbx"); got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})
}
