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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)

	ociChartName := "oci-depending-chart"
	c := createTestingMetadataForOCI(ociChartName, ociSrv.RegistryURL)
	_, err = chartutil.Save(c, ociSrv.Dir)
	require.NoError(t, err)
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

	assert.Contains(t, out, `update from the "test" chart repository`, "Repo did not get updated\n%s", out)

	// Make sure the actual file got downloaded.
	expect := filepath.Join(rootDir, chartname, "charts", "reqtest-0.1.0.tgz")
	_, err = os.Stat(expect)
	require.NoError(t, err)

	// In the second pass, we want to remove the chart's request dependency,
	// then see if it restores from the lock.
	lockfile := filepath.Join(rootDir, chartname, "Chart.lock")
	_, err = os.Stat(lockfile)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(expect))

	_, out, err = executeActionCommand(cmd)
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// Now repeat the test that the dependency exists.
	_, err = os.Stat(expect)
	require.NoError(t, err)

	// Make sure that build is also fetching the correct version.
	hash, err := provenance.DigestFile(expect)
	require.NoError(t, err)

	i, err := repo.LoadIndexFile(filepath.Join(rootDir, "index.yaml"))
	require.NoError(t, err)

	reqver := i.Entries["reqtest"][0]
	h := reqver.Digest
	assert.Equalf(t, h, hash, "Failed hash match: expected %s, got %s", hash, h)
	v := reqver.Version
	assert.Equalf(t, "0.1.0", v, "mismatched versions. Expected %q, got %q", "0.1.0", v)

	skipRefreshCmd := fmt.Sprintf("dependency build '%s' --skip-refresh --repository-config %s --repository-cache %s --plain-http", filepath.Join(rootDir, chartname), repoFile, rootDir)
	_, out, err = executeActionCommand(skipRefreshCmd)

	// In this pass, we check --skip-refresh option becomes effective.
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	assert.NotContains(t, out, `update from the "test" chart repository`, "Repo did get updated\n%s", out)

	// OCI dependencies
	require.NoError(t, chartutil.SaveDir(c, dir()))
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
	_, err = os.Stat(expect)
	require.NoError(t, err)
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
