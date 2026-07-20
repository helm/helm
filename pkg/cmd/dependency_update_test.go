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
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/repo/v1"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestDependencyUpdateCmd(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz"),
	)
	defer srv.Stop()
	t.Logf("Listening on directory %s", srv.Root())

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	require.NoError(t, err)
	contentCache := t.TempDir()

	ociChartName := "oci-depending-chart"
	c := createTestingMetadataForOCI(ociChartName, ociSrv.RegistryURL)
	_, err = chartutil.Save(c, ociSrv.Dir)
	require.NoError(t, err)
	ociSrv.Run(t, repotest.WithDependingChart(c))

	require.NoError(t, srv.LinkIndices())

	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}

	chartname := "depup"
	ch := createTestingMetadata(chartname, srv.URL())
	md := ch.Metadata
	require.NoError(t, chartutil.SaveDir(ch, dir()))

	_, out, err := executeActionCommand(
		fmt.Sprintf("dependency update '%s' --repository-config %s --repository-cache %s --content-cache %s --plain-http", dir(chartname), dir("repositories.yaml"), dir(), contentCache),
	)
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// This is written directly to stdout, so we have to capture as is.
	assert.Contains(t, out, `update from the "test" chart repository`, "Repo did not get updated\n%s", out)

	// Make sure the actual file got downloaded.
	expect := dir(chartname, "charts/reqtest-0.1.0.tgz")
	_, err = os.Stat(expect)
	require.NoError(t, err)

	hash, err := provenance.DigestFile(expect)
	require.NoError(t, err)

	i, err := repo.LoadIndexFile(dir(helmpath.CacheIndexFile("test")))
	require.NoError(t, err)

	reqver := i.Entries["reqtest"][0]
	h := reqver.Digest
	assert.Equalf(t, h, hash, "Failed hash match: expected %s, got %s", hash, h)

	// Now change the dependencies and update. This verifies that on update,
	// old dependencies are cleansed and new dependencies are added.
	md.Dependencies = []*chart.Dependency{
		{Name: "reqtest", Version: "0.1.0", Repository: srv.URL()},
		{Name: "compressedchart", Version: "0.3.0", Repository: srv.URL()},
	}
	require.NoError(t, chartutil.SaveChartfile(dir(chartname, "Chart.yaml"), md))

	_, out, err = executeActionCommand(fmt.Sprintf("dependency update '%s' --repository-config %s --repository-cache %s --content-cache %s --plain-http", dir(chartname), dir("repositories.yaml"), dir(), contentCache))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// In this second run, we should see compressedchart-0.3.0.tgz, and not
	// the 0.1.0 version.
	expect = dir(chartname, "charts/compressedchart-0.3.0.tgz")
	_, err = os.Stat(expect)
	require.NoErrorf(t, err, "Expected %q", expect)
	unexpected := dir(chartname, "charts/compressedchart-0.1.0.tgz")
	_, err = os.Stat(unexpected)
	require.Errorf(t, err, "Unexpected %q", unexpected)

	// test for OCI charts
	require.NoError(t, chartutil.SaveDir(c, dir()))
	cmd := fmt.Sprintf("dependency update '%s' --repository-config %s --repository-cache %s --registry-config %s/config.json --content-cache %s --plain-http",
		dir(ociChartName),
		dir("repositories.yaml"),
		dir(),
		dir(),
		contentCache)
	_, out, err = executeActionCommand(cmd)
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
	expect = dir(ociChartName, "charts/oci-dependent-chart-0.1.0.tgz")
	_, err = os.Stat(expect)
	require.NoError(t, err)
}

func TestDependencyUpdateCmd_DoNotDeleteOldChartsOnError(t *testing.T) {
	defer resetEnv()()
	ensure.HelmHome(t)

	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz"),
	)
	defer srv.Stop()
	t.Logf("Listening on directory %s", srv.Root())

	require.NoError(t, srv.LinkIndices())

	chartname := "depupdelete"

	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}
	createTestingChart(t, dir(), chartname, srv.URL())

	_, output, err := executeActionCommand(fmt.Sprintf("dependency update %s --repository-config %s --repository-cache %s --plain-http", dir(chartname), dir("repositories.yaml"), dir()))
	if err != nil {
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	// Chart repo is down
	srv.Stop()
	contentCache := t.TempDir()

	_, output, err = executeActionCommand(fmt.Sprintf("dependency update %s --repository-config %s --repository-cache %s --content-cache %s --plain-http", dir(chartname), dir("repositories.yaml"), dir(), contentCache))
	if err == nil {
		t.Logf("Output: %s", output)
		t.Fatal("Expected error, got nil")
	}

	// Make sure charts dir still has dependencies
	files, err := os.ReadDir(filepath.Join(dir(chartname), "charts"))
	require.NoError(t, err)
	dependencies := []string{"compressedchart-0.1.0.tgz", "reqtest-0.1.0.tgz"}

	require.Len(t, dependencies, len(files), "Expected %d chart dependencies, got %d", len(dependencies), len(files))
	for index, file := range files {
		require.Equal(t, file.Name(), dependencies[index], "Chart dependency %s not matching %s", dependencies[index], file.Name())
	}

	// Make sure tmpcharts-x is deleted
	tmpPath := filepath.Join(dir(chartname), fmt.Sprintf("tmpcharts-%d", os.Getpid()))
	_, err = os.Stat(tmpPath)
	require.ErrorIs(t, err, fs.ErrNotExist, "tmpcharts dir still exists")
}

func TestDependencyUpdateCmd_WithRepoThatWasNotAdded(t *testing.T) {
	srv := setupMockRepoServer(t)
	srvForUnmanagedRepo := setupMockRepoServer(t)
	defer srv.Stop()
	defer srvForUnmanagedRepo.Stop()

	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}

	chartname := "depup"
	ch := createTestingMetadata(chartname, srv.URL())
	chartDependency := &chart.Dependency{
		Name:       "signtest",
		Version:    "0.1.0",
		Repository: srvForUnmanagedRepo.URL(),
	}
	ch.Metadata.Dependencies = append(ch.Metadata.Dependencies, chartDependency)

	require.NoError(t, chartutil.SaveDir(ch, dir()))

	contentCache := t.TempDir()

	_, out, err := executeActionCommand(
		fmt.Sprintf("dependency update '%s' --repository-config %s --repository-cache %s --content-cache %s", dir(chartname),
			dir("repositories.yaml"), dir(), contentCache),
	)

	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// This is written directly to stdout, so we have to capture as is
	assert.Contains(t, out, `Getting updates for unmanaged Helm repositories...`, "No ‘unmanaged’ Helm repo used in test chartdependency or it doesn’t cause the creation "+
		"of an ‘ad hoc’ repo index cache file\n%s", out)
}

func setupMockRepoServer(t *testing.T) *repotest.Server {
	t.Helper()
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz"),
	)

	t.Logf("Listening on directory %s", srv.Root())

	require.NoError(t, srv.LinkIndices())

	return srv
}

// createTestingMetadata creates a basic chart that depends on reqtest-0.1.0
//
// The baseURL can be used to point to a particular repository server.
func createTestingMetadata(name, baseURL string) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       name,
			Version:    "1.2.3",
			Dependencies: []*chart.Dependency{
				{Name: "reqtest", Version: "0.1.0", Repository: baseURL},
				{Name: "compressedchart", Version: "0.1.0", Repository: baseURL},
			},
		},
	}
}

func createTestingMetadataForOCI(name, registryURL string) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       name,
			Version:    "1.2.3",
			Dependencies: []*chart.Dependency{
				{Name: "oci-dependent-chart", Version: "0.1.0", Repository: fmt.Sprintf("oci://%s/u/ocitestuser", registryURL)},
			},
		},
	}
}

// createTestingChart creates a basic chart that depends on reqtest-0.1.0
//
// The baseURL can be used to point to a particular repository server.
func createTestingChart(t *testing.T, dest, name, baseURL string) {
	t.Helper()
	cfile := createTestingMetadata(name, baseURL)
	require.NoError(t, chartutil.SaveDir(cfile, dest))
}
