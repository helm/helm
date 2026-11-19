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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestUpgradeCmd(t *testing.T) {
	tmpChart := t.TempDir()
	cfile := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion:  chart.APIVersionV1,
			Name:        "testUpgradeChart",
			Description: "A Helm chart for Kubernetes",
			Version:     "0.1.0",
		},
	}
	chartPath := filepath.Join(tmpChart, cfile.Metadata.Name)
	require.NoErrorf(t, chartutil.SaveDir(cfile, tmpChart), "Error creating chart for upgrade")
	ch, err := loader.Load(chartPath)
	require.NoError(t, err, "Error loading chart")
	_ = release.Mock(&release.MockReleaseOptions{
		Name:  "funny-bunny",
		Chart: ch,
	})

	// update chart version
	cfile.Metadata.Version = "0.1.2"

	require.NoErrorf(t, chartutil.SaveDir(cfile, tmpChart), "Error creating chart")
	ch, err = loader.Load(chartPath)
	require.NoError(t, err, "Error loading updated chart")

	// update chart version again
	cfile.Metadata.Version = "0.1.3"

	require.NoErrorf(t, chartutil.SaveDir(cfile, tmpChart), "Error creating chart")
	var ch2 *chart.Chart
	ch2, err = loader.Load(chartPath)
	require.NoError(t, err, "Error loading updated chart")

	missingDepsPath := "testdata/testcharts/chart-missing-deps"
	badDepsPath := "testdata/testcharts/chart-bad-requirements"
	presentDepsPath := "testdata/testcharts/chart-with-subchart-update"

	relWithStatusMock := func(n string, v int, ch *chart.Chart, status rcommon.Status) *release.Release {
		return release.Mock(&release.MockReleaseOptions{Name: n, Version: v, Chart: ch, Status: status})
	}

	relMock := func(n string, v int, ch *chart.Chart) *release.Release {
		return release.Mock(&release.MockReleaseOptions{Name: n, Version: v, Chart: ch})
	}

	tests := []cmdTestCase{
		{
			name:   "upgrade a release",
			cmd:    fmt.Sprintf("upgrade funny-bunny '%s'", chartPath),
			golden: "output/upgrade.txt",
			rels:   []*release.Release{relMock("funny-bunny", 2, ch)},
		},
		{
			name:   "upgrade a release with timeout",
			cmd:    fmt.Sprintf("upgrade funny-bunny --timeout 120s '%s'", chartPath),
			golden: "output/upgrade-with-timeout.txt",
			rels:   []*release.Release{relMock("funny-bunny", 3, ch2)},
		},
		{
			name:   "upgrade a release with --reset-values",
			cmd:    fmt.Sprintf("upgrade funny-bunny --reset-values '%s'", chartPath),
			golden: "output/upgrade-with-reset-values.txt",
			rels:   []*release.Release{relMock("funny-bunny", 4, ch2)},
		},
		{
			name:   "upgrade a release with --reuse-values",
			cmd:    fmt.Sprintf("upgrade funny-bunny --reuse-values '%s'", chartPath),
			golden: "output/upgrade-with-reset-values2.txt",
			rels:   []*release.Release{relMock("funny-bunny", 5, ch2)},
		},
		{
			name:   "upgrade a release with --take-ownership",
			cmd:    fmt.Sprintf("upgrade funny-bunny '%s' --take-ownership", chartPath),
			golden: "output/upgrade-and-take-ownership.txt",
			rels:   []*release.Release{relMock("funny-bunny", 2, ch)},
		},
		{
			name:   "install a release with 'upgrade --install'",
			cmd:    fmt.Sprintf("upgrade zany-bunny -i '%s'", chartPath),
			golden: "output/upgrade-with-install.txt",
			rels:   []*release.Release{relMock("zany-bunny", 1, ch)},
		},
		{
			name:   "install a release with 'upgrade --install' and timeout",
			cmd:    fmt.Sprintf("upgrade crazy-bunny -i --timeout 120s '%s'", chartPath),
			golden: "output/upgrade-with-install-timeout.txt",
			rels:   []*release.Release{relMock("crazy-bunny", 1, ch)},
		},
		{
			name:   "upgrade a release with wait",
			cmd:    fmt.Sprintf("upgrade crazy-bunny --wait '%s'", chartPath),
			golden: "output/upgrade-with-wait.txt",
			rels:   []*release.Release{relMock("crazy-bunny", 2, ch2)},
		},
		{
			name:   "upgrade a release with wait-for-jobs",
			cmd:    fmt.Sprintf("upgrade crazy-bunny --wait --wait-for-jobs '%s'", chartPath),
			golden: "output/upgrade-with-wait-for-jobs.txt",
			rels:   []*release.Release{relMock("crazy-bunny", 2, ch2)},
		},
		{
			name:      "upgrade a release with missing dependencies",
			cmd:       "upgrade bonkers-bunny " + missingDepsPath,
			golden:    "output/upgrade-with-missing-dependencies.txt",
			wantError: true,
		},
		{
			name:      "upgrade a release with bad dependencies",
			cmd:       fmt.Sprintf("upgrade bonkers-bunny '%s'", badDepsPath),
			golden:    "output/upgrade-with-bad-dependencies.txt",
			wantError: true,
		},
		{
			name:   "upgrade a release with resolving missing dependencies",
			cmd:    "upgrade --dependency-update funny-bunny " + presentDepsPath,
			golden: "output/upgrade-with-dependency-update.txt",
			rels:   []*release.Release{relMock("funny-bunny", 2, ch2)},
		},
		{
			name:      "upgrade a non-existent release",
			cmd:       fmt.Sprintf("upgrade funny-bunny '%s'", chartPath),
			golden:    "output/upgrade-with-bad-or-missing-existing-release.txt",
			wantError: true,
		},
		{
			name:   "upgrade a failed release",
			cmd:    fmt.Sprintf("upgrade funny-bunny '%s'", chartPath),
			golden: "output/upgrade.txt",
			rels:   []*release.Release{relWithStatusMock("funny-bunny", 2, ch, rcommon.StatusFailed)},
		},
		{
			name:      "upgrade a pending install release",
			cmd:       fmt.Sprintf("upgrade funny-bunny '%s'", chartPath),
			golden:    "output/upgrade-with-pending-install.txt",
			wantError: true,
			rels:      []*release.Release{relWithStatusMock("funny-bunny", 2, ch, rcommon.StatusPendingInstall)},
		},
		{
			name:   "install a previously uninstalled release with '--keep-history' using 'upgrade --install'",
			cmd:    fmt.Sprintf("upgrade funny-bunny -i '%s'", chartPath),
			golden: "output/upgrade-uninstalled-with-keep-history.txt",
			rels:   []*release.Release{relWithStatusMock("funny-bunny", 2, ch, rcommon.StatusUninstalled)},
		},
	}
	runTestCmd(t, tests)
}

// TestUpgradeDependencyUpdateOCINoPanic is a regression test for a nil-pointer
// panic in `helm upgrade --dependency-update` when a chart declares an OCI
// dependency. The upgrade command built its downloader.Manager without a
// RegistryClient (unlike install, dependency update, and dependency build), so
// resolving an OCI dependency dereferenced a nil *registry.Client. The command
// must now return a graceful error instead of panicking.
func TestUpgradeDependencyUpdateOCINoPanic(t *testing.T) {
	defer resetEnv()()

	// A stub registry that answers the API-version ping but rejects the tag
	// lookup, so OCI dependency resolution fails fast and hermetically instead
	// of reaching a real registry.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// A chart with an unresolved OCI dependency forces --dependency-update into
	// the tag-lookup path that previously panicked: the version is a range (an
	// explicit version would skip the lookup) and the dependency is not present
	// under charts/.
	tmp := t.TempDir()
	parent := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "oci-parent",
			Version:    "0.1.0",
			Dependencies: []*chart.Dependency{{
				Name:       "subchart",
				Repository: fmt.Sprintf("oci://%s/charts", srv.Listener.Addr()),
				Version:    "^1.0.0",
			}},
		},
	}
	require.NoError(t, chartutil.SaveDir(parent, tmp), "Error creating chart")
	chartPath := filepath.Join(tmp, parent.Metadata.Name)
	// SaveDir writes only resolved subcharts (Chart.Dependencies()), not the
	// declared Metadata.Dependencies, so create the empty charts/ directory
	// explicitly to make the "dependency missing from charts/" state concrete.
	require.NoError(t, os.MkdirAll(filepath.Join(chartPath, "charts"), 0o755), "Error creating charts dir")

	// The command must return an error (registry rejects the lookup), not panic.
	_, _, err := executeActionCommandC(storageFixture(),
		fmt.Sprintf("upgrade --dependency-update --plain-http oci-parent '%s'", chartPath))
	require.Error(t, err, "expected an error resolving the OCI dependency, got nil")
}

func TestUpgradeWithValue(t *testing.T) {
	releaseName := "funny-bunny-v2"
	relMock, ch, chartPath := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	store.Create(relMock(releaseName, 3, ch))

	cmd := fmt.Sprintf("upgrade %s --set favoriteDrink=tea '%s'", releaseName, chartPath)
	_, _, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	updatedReli, err := store.Get(releaseName, 4)
	require.NoError(t, err)

	updatedRel, err := releaserToV1Release(updatedReli)
	require.NoError(t, err)
	assert.Contains(t, updatedRel.Manifest, "drink: tea", "The value is not set correctly. manifest: %s", updatedRel.Manifest)
}

func TestUpgradeWithStringValue(t *testing.T) {
	releaseName := "funny-bunny-v3"
	relMock, ch, chartPath := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	store.Create(relMock(releaseName, 3, ch))

	cmd := fmt.Sprintf("upgrade %s --set-string favoriteDrink=coffee '%s'", releaseName, chartPath)
	_, _, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	updatedReli, err := store.Get(releaseName, 4)
	require.NoError(t, err)

	updatedRel, err := releaserToV1Release(updatedReli)
	require.NoError(t, err)
	assert.Contains(t, updatedRel.Manifest, "drink: coffee", "The value is not set correctly. manifest: %s", updatedRel.Manifest)
}

func TestUpgradeInstallWithSubchartNotes(t *testing.T) {
	releaseName := "wacky-bunny-v1"
	relMock, ch, _ := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	store.Create(relMock(releaseName, 1, ch))

	cmd := fmt.Sprintf("upgrade %s -i --render-subchart-notes '%s'", releaseName, "testdata/testcharts/chart-with-subchart-notes")
	_, _, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	upgradedReli, err := store.Get(releaseName, 2)
	require.NoError(t, err)

	upgradedRel, err := releaserToV1Release(upgradedReli)
	require.NoError(t, err)
	assert.Contains(t, upgradedRel.Info.Notes, "PARENT NOTES", "The parent notes are not set correctly. NOTES: %s", upgradedRel.Info.Notes)
	assert.Contains(t, upgradedRel.Info.Notes, "SUBCHART NOTES", "The subchart notes are not set correctly. NOTES: %s", upgradedRel.Info.Notes)
}

func TestUpgradeWithValuesFile(t *testing.T) {
	releaseName := "funny-bunny-v4"
	relMock, ch, chartPath := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	store.Create(relMock(releaseName, 3, ch))

	cmd := fmt.Sprintf("upgrade %s --values testdata/testcharts/upgradetest/values.yaml '%s'", releaseName, chartPath)
	_, _, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	updatedReli, err := store.Get(releaseName, 4)
	require.NoError(t, err)

	updatedRel, err := releaserToV1Release(updatedReli)
	require.NoError(t, err)
	assert.Contains(t, updatedRel.Manifest, "drink: beer", "The value is not set correctly. manifest: %s", updatedRel.Manifest)
}

func TestUpgradeWithValuesFromStdin(t *testing.T) {
	releaseName := "funny-bunny-v5"
	relMock, ch, chartPath := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	store.Create(relMock(releaseName, 3, ch))

	in, err := os.Open("testdata/testcharts/upgradetest/values.yaml")
	require.NoError(t, err)

	cmd := fmt.Sprintf("upgrade %s --values - '%s'", releaseName, chartPath)
	_, _, err = executeActionCommandStdinC(store, in, cmd)
	require.NoError(t, err)

	updatedReli, err := store.Get(releaseName, 4)
	require.NoError(t, err)

	updatedRel, err := releaserToV1Release(updatedReli)
	require.NoError(t, err)
	assert.Contains(t, updatedRel.Manifest, "drink: beer", "The value is not set correctly. manifest: %s", updatedRel.Manifest)
}

func TestUpgradeInstallWithValuesFromStdin(t *testing.T) {
	releaseName := "funny-bunny-v6"
	_, _, chartPath := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	in, err := os.Open("testdata/testcharts/upgradetest/values.yaml")
	require.NoError(t, err)

	cmd := fmt.Sprintf("upgrade %s -f - --install '%s'", releaseName, chartPath)
	_, _, err = executeActionCommandStdinC(store, in, cmd)
	require.NoError(t, err)

	updatedReli, err := store.Get(releaseName, 1)
	require.NoError(t, err)

	updatedRel, err := releaserToV1Release(updatedReli)
	require.NoError(t, err)
	assert.Contains(t, updatedRel.Manifest, "drink: beer", "The value is not set correctly. manifest: %s", updatedRel.Manifest)
}

func prepareMockRelease(t *testing.T, releaseName string) (func(n string, v int, ch *chart.Chart) *release.Release, *chart.Chart, string) {
	t.Helper()
	tmpChart := t.TempDir()
	configmapData, err := os.ReadFile("testdata/testcharts/upgradetest/templates/configmap.yaml")
	require.NoError(t, err, "Error loading template yaml")
	cfile := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion:  chart.APIVersionV1,
			Name:        "testUpgradeChart",
			Description: "A Helm chart for Kubernetes",
			Version:     "0.1.0",
		},
		Templates: []*common.File{{Name: "templates/configmap.yaml", ModTime: time.Now(), Data: configmapData}},
	}
	chartPath := filepath.Join(tmpChart, cfile.Metadata.Name)
	require.NoErrorf(t, chartutil.SaveDir(cfile, tmpChart), "Error creating chart for upgrade")
	ch, err := loader.Load(chartPath)
	require.NoError(t, err, "Error loading chart")
	_ = release.Mock(&release.MockReleaseOptions{
		Name:  releaseName,
		Chart: ch,
	})

	relMock := func(n string, v int, ch *chart.Chart) *release.Release {
		return release.Mock(&release.MockReleaseOptions{Name: n, Version: v, Chart: ch})
	}

	return relMock, ch, chartPath
}

func TestUpgradeOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "upgrade")
}

func TestUpgradeVersionCompletion(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	tests := []cmdTestCase{{
		name:   "completion for upgrade version flag",
		cmd:    repoSetup + " __complete upgrade releasename testing/alpine --version ''",
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for upgrade version flag, no filter",
		cmd:    repoSetup + " __complete upgrade releasename testing/alpine --version 0.3",
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for upgrade version flag too few args",
		cmd:    repoSetup + " __complete upgrade releasename --version ''",
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for upgrade version flag too many args",
		cmd:    repoSetup + " __complete upgrade releasename testing/alpine badarg --version ''",
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for upgrade version flag invalid chart",
		cmd:    repoSetup + " __complete upgrade releasename invalid/invalid --version ''",
		golden: "output/version-invalid-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestUpgradeFileCompletion(t *testing.T) {
	checkFileCompletion(t, "upgrade", false)
	checkFileCompletion(t, "upgrade myrelease", true)
	checkFileCompletion(t, "upgrade myrelease repo/chart", false)
}

func TestUpgradeInstallWithLabels(t *testing.T) {
	releaseName := "funny-bunny-labels"
	_, _, chartPath := prepareMockRelease(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	expectedLabels := map[string]string{
		"key1": "val1",
		"key2": "val2",
	}
	cmd := fmt.Sprintf("upgrade %s --install --labels key1=val1,key2=val2 '%s'", releaseName, chartPath)
	_, _, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	updatedReli, err := store.Get(releaseName, 1)
	require.NoError(t, err)

	updatedRel, err := releaserToV1Release(updatedReli)
	require.NoError(t, err)
	assert.Equal(t, expectedLabels, updatedRel.Labels)
}

func prepareMockReleaseWithSecret(t *testing.T, releaseName string) (func(n string, v int, ch *chart.Chart) *release.Release, *chart.Chart, string) {
	t.Helper()
	tmpChart := t.TempDir()
	configmapData, err := os.ReadFile("testdata/testcharts/chart-with-secret/templates/configmap.yaml")
	require.NoError(t, err, "Error loading template yaml")
	secretData, err := os.ReadFile("testdata/testcharts/chart-with-secret/templates/secret.yaml")
	require.NoError(t, err, "Error loading template yaml")
	modTime := time.Now()
	cfile := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion:  chart.APIVersionV1,
			Name:        "testUpgradeChart",
			Description: "A Helm chart for Kubernetes",
			Version:     "0.1.0",
		},
		Templates: []*common.File{{Name: "templates/configmap.yaml", ModTime: modTime, Data: configmapData}, {Name: "templates/secret.yaml", ModTime: modTime, Data: secretData}},
	}
	chartPath := filepath.Join(tmpChart, cfile.Metadata.Name)
	require.NoErrorf(t, chartutil.SaveDir(cfile, tmpChart), "Error creating chart for upgrade")
	ch, err := loader.Load(chartPath)
	require.NoError(t, err, "Error loading chart")
	_ = release.Mock(&release.MockReleaseOptions{
		Name:  releaseName,
		Chart: ch,
	})

	relMock := func(n string, v int, ch *chart.Chart) *release.Release {
		return release.Mock(&release.MockReleaseOptions{Name: n, Version: v, Chart: ch})
	}

	return relMock, ch, chartPath
}

func TestUpgradeWithDryRun(t *testing.T) {
	releaseName := "funny-bunny-labels"
	_, _, chartPath := prepareMockReleaseWithSecret(t, releaseName)

	defer resetEnv()()

	store := storageFixture()

	// First install a release into the store so that future --dry-run attempts
	// have it available.
	cmd := fmt.Sprintf("upgrade %s --install '%s'", releaseName, chartPath)
	_, _, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	_, err = store.Get(releaseName, 1)
	require.NoError(t, err)

	cmd = fmt.Sprintf("upgrade %s --dry-run '%s'", releaseName, chartPath)
	_, out, err := executeActionCommandC(store, cmd)
	require.NoError(t, err)

	// No second release should be stored because this is a dry run.
	_, err = store.Get(releaseName, 2)
	require.Error(t, err, "expected error as there should be no new release but got none")
	assert.Contains(t, out, "kind: Secret", "expected secret in output from --dry-run but found none")

	// Ensure the secret is not in the output
	cmd = fmt.Sprintf("upgrade %s --dry-run --hide-secret '%s'", releaseName, chartPath)
	_, out, err = executeActionCommandC(store, cmd)
	require.NoError(t, err)

	// No second release should be stored because this is a dry run.
	_, err = store.Get(releaseName, 2)
	require.Error(t, err, "expected error as there should be no new release but got none")
	assert.NotContains(t, out, "kind: Secret", "expected no secret in output from --dry-run --hide-secret but found one")

	// Ensure there is an error when --hide-secret used without dry-run
	cmd = fmt.Sprintf("upgrade %s --hide-secret '%s'", releaseName, chartPath)
	_, _, err = executeActionCommandC(store, cmd)
	assert.Error(t, err, "expected error when --hide-secret used without --dry-run")
}

func TestUpgradeInstallServerSideApply(t *testing.T) {
	_, _, chartPath := prepareMockRelease(t, "ssa-test")

	defer resetEnv()()

	tests := []struct {
		name                string
		serverSideFlag      string
		expectedApplyMethod string
	}{
		{
			name:                "upgrade --install with --server-side=false uses client-side apply",
			serverSideFlag:      "--server-side=false",
			expectedApplyMethod: "csa",
		},
		{
			name:                "upgrade --install with --server-side=true uses server-side apply",
			serverSideFlag:      "--server-side=true",
			expectedApplyMethod: "ssa",
		},
		{
			name:                "upgrade --install with --server-side=auto uses server-side apply (default for new install)",
			serverSideFlag:      "--server-side=auto",
			expectedApplyMethod: "ssa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storageFixture()
			releaseName := "ssa-test-" + tt.expectedApplyMethod

			cmd := fmt.Sprintf("upgrade %s --install %s '%s'", releaseName, tt.serverSideFlag, chartPath)
			_, _, err := executeActionCommandC(store, cmd)
			require.NoError(t, err)

			rel, err := store.Get(releaseName, 1)
			require.NoError(t, err, "unexpected error getting release")

			relV1, err := releaserToV1Release(rel)
			require.NoError(t, err, "unexpected error converting release")
			assert.Equal(t, tt.expectedApplyMethod, relV1.ApplyMethod, "expected ApplyMethod %q, got %q", tt.expectedApplyMethod, relV1.ApplyMethod)
		})
	}
}
