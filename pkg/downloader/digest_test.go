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

package downloader

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/internal/resolver"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func setupDigestTestManager(t *testing.T, chartPath string, srv *repotest.Server) (*Manager, *bytes.Buffer) {
	t.Helper()
	out := bytes.NewBuffer(nil)
	contentCache := t.TempDir()
	m := &Manager{
		ChartPath:        chartPath,
		Out:              out,
		Getters:          getter.Providers{{Schemes: []string{"http", "https"}, New: getter.NewHTTPGetter}},
		RepositoryConfig: filepath.Join(srv.Root(), "repositories.yaml"),
		RepositoryCache:  srv.Root(),
		ContentCache:     contentCache,
	}
	return m, out
}

func setupParentChart(t *testing.T, srv *repotest.Server, dep *chart.Dependency) (string, *chart.Chart) {
	t.Helper()
	dir := func(p ...string) string {
		return filepath.Join(append([]string{srv.Root()}, p...)...)
	}
	parent := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         "parent",
			Version:      "0.1.0",
			APIVersion:   "v2",
			Dependencies: []*chart.Dependency{dep},
		},
	}
	require.NoError(t, chartutil.SaveDir(parent, dir()))
	return dir("parent"), parent
}

func TestContentDigestUpdateOnRepublish(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()
	require.NoError(t, srv.LinkIndices())

	parentPath, _ := setupParentChart(t, srv, &chart.Dependency{
		Name:       "local-subchart",
		Version:    "0.1.0",
		Repository: srv.URL(),
	})

	m, _ := setupDigestTestManager(t, parentPath, srv)
	require.NoError(t, m.Update())

	lockData, err := os.ReadFile(filepath.Join(parentPath, "Chart.lock"))
	require.NoError(t, err)
	var lock1 chart.Lock
	require.NoError(t, yaml.Unmarshal(lockData, &lock1))
	firstDigest := lock1.Dependencies[0].Digest
	require.NotEmpty(t, firstDigest)
	assert.True(t, strings.HasPrefix(firstDigest, "sha256:"))

	// Republish same version with different chart bytes.
	sub, err := loader.LoadDir(filepath.Join("testdata", "local-subchart"))
	require.NoError(t, err)
	sub.Metadata.Description = "modified content"
	_, err = chartutil.Save(sub, srv.Root())
	require.NoError(t, err)
	require.NoError(t, srv.CreateIndex())

	require.NoError(t, m.Update())

	lockData, err = os.ReadFile(filepath.Join(parentPath, "Chart.lock"))
	require.NoError(t, err)
	var lock2 chart.Lock
	require.NoError(t, yaml.Unmarshal(lockData, &lock2))
	secondDigest := lock2.Dependencies[0].Digest
	require.NotEmpty(t, secondDigest)
	assert.NotEqual(t, firstDigest, secondDigest)
	assert.Equal(t, lock1.Digest, lock2.Digest, "metadata digest should be unchanged when version pin is exact")
}

func TestContentDigestStableOnSameBytes(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()
	require.NoError(t, srv.LinkIndices())

	parentPath, _ := setupParentChart(t, srv, &chart.Dependency{
		Name:       "local-subchart",
		Version:    "0.1.0",
		Repository: srv.URL(),
	})

	m, _ := setupDigestTestManager(t, parentPath, srv)
	require.NoError(t, m.Update())

	lockBefore, err := os.ReadFile(filepath.Join(parentPath, "Chart.lock"))
	require.NoError(t, err)

	require.NoError(t, m.Update())

	lockAfter, err := os.ReadFile(filepath.Join(parentPath, "Chart.lock"))
	require.NoError(t, err)
	assert.Equal(t, string(lockBefore), string(lockAfter))
}

func TestBuildDigestMismatch(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()
	require.NoError(t, srv.LinkIndices())

	parentPath, _ := setupParentChart(t, srv, &chart.Dependency{
		Name:       "local-subchart",
		Version:    "0.1.0",
		Repository: srv.URL(),
	})

	m, _ := setupDigestTestManager(t, parentPath, srv)
	require.NoError(t, m.Update())

	sub, err := loader.LoadDir(filepath.Join("testdata", "local-subchart"))
	require.NoError(t, err)
	sub.Metadata.Description = "tampered upstream content"
	_, err = chartutil.Save(sub, srv.Root())
	require.NoError(t, err)
	require.NoError(t, srv.CreateIndex())

	err = m.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart.lock digest mismatch for local-subchart-0.1.0")
}

func TestLegacyLockWithoutDigest(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()
	require.NoError(t, srv.LinkIndices())

	parentPath, parent := setupParentChart(t, srv, &chart.Dependency{
		Name:       "local-subchart",
		Version:    "0.1.0",
		Repository: srv.URL(),
	})

	legacyLock := &chart.Lock{
		Generated: time.Now(),
		Dependencies: []*chart.Dependency{{
			Name:       "local-subchart",
			Version:    "0.1.0",
			Repository: srv.URL(),
		}},
	}
	digest, err := resolver.HashReq(parent.Metadata.Dependencies, legacyLock.Dependencies)
	require.NoError(t, err)
	legacyLock.Digest = digest
	lockData, err := yaml.Marshal(legacyLock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(parentPath, "Chart.lock"), lockData, 0644))

	m, _ := setupDigestTestManager(t, parentPath, srv)
	require.NoError(t, m.Build())

	require.NoError(t, m.Update())

	lockData, err = os.ReadFile(filepath.Join(parentPath, "Chart.lock"))
	require.NoError(t, err)
	var updated chart.Lock
	require.NoError(t, yaml.Unmarshal(lockData, &updated))
	require.NotEmpty(t, updated.Dependencies[0].Digest)
}

func TestIndexDigestMismatchWarning(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()
	require.NoError(t, srv.LinkIndices())

	depArchive := filepath.Join(srv.Root(), "local-subchart-0.1.0.tgz")
	realDigest, err := provenance.DigestFile(depArchive)
	require.NoError(t, err)

	dest := t.TempDir()
	contentCache := t.TempDir()
	out := bytes.NewBuffer(nil)
	dl := ChartDownloader{
		Out:              out,
		RepositoryConfig: filepath.Join(srv.Root(), "repositories.yaml"),
		RepositoryCache:  srv.Root(),
		ContentCache:     contentCache,
		Getters:          getter.Providers{{Schemes: []string{"http", "https"}, New: getter.NewHTTPGetter}},
		ExpectedDigest:   strings.Repeat("a", len(realDigest)),
	}

	_, _, err = dl.DownloadTo(srv.URL()+"/local-subchart-0.1.0.tgz", "0.1.0", dest)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "WARNING: digest mismatch")
}

func TestSemverRangeRecordsContentDigest(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()

	sub, err := loader.LoadDir(filepath.Join("testdata", "local-subchart"))
	require.NoError(t, err)
	sub.Metadata.Version = "0.2.0"
	_, err = chartutil.Save(sub, srv.Root())
	require.NoError(t, err)
	require.NoError(t, srv.CreateIndex())
	require.NoError(t, srv.LinkIndices())

	parentPath, _ := setupParentChart(t, srv, &chart.Dependency{
		Name:       "local-subchart",
		Version:    ">=0.1.0",
		Repository: srv.URL(),
	})

	m, _ := setupDigestTestManager(t, parentPath, srv)
	require.NoError(t, m.Update())

	lockData, err := os.ReadFile(filepath.Join(parentPath, "Chart.lock"))
	require.NoError(t, err)
	var lock chart.Lock
	require.NoError(t, yaml.Unmarshal(lockData, &lock))
	require.Len(t, lock.Dependencies, 1)
	assert.Equal(t, "0.2.0", lock.Dependencies[0].Version)
	require.NotEmpty(t, lock.Dependencies[0].Digest)

	tgzPath := filepath.Join(parentPath, "charts", "local-subchart-0.2.0.tgz")
	require.FileExists(t, tgzPath)
	got, err := provenance.DigestFile(tgzPath)
	require.NoError(t, err)
	assert.Equal(t, "sha256:"+got, lock.Dependencies[0].Digest)
}

func TestLockNeedsWrite(t *testing.T) {
	metaDigest := "sha256:abc"
	old := &chart.Lock{
		Digest: metaDigest,
		Dependencies: []*chart.Dependency{{
			Name:    "foo",
			Version: "1.0.0",
			Digest:  "sha256:111",
		}},
	}
	same := &chart.Lock{
		Digest: metaDigest,
		Dependencies: []*chart.Dependency{{
			Name:    "foo",
			Version: "1.0.0",
			Digest:  "sha256:111",
		}},
	}
	changedContent := &chart.Lock{
		Digest: metaDigest,
		Dependencies: []*chart.Dependency{{
			Name:    "foo",
			Version: "1.0.0",
			Digest:  "sha256:222",
		}},
	}
	assert.False(t, lockNeedsWrite(old, same))
	assert.True(t, lockNeedsWrite(old, changedContent))
	assert.True(t, lockNeedsWrite(nil, same))

	withNil := &chart.Lock{
		Digest: metaDigest,
		Dependencies: []*chart.Dependency{
			nil,
			{Name: "foo", Version: "1.0.0", Digest: "sha256:111"},
		},
	}
	assert.True(t, lockNeedsWrite(old, withNil))
	assert.True(t, dependencyDigestsEqual(withNil.Dependencies, withNil.Dependencies))
}

func TestLoadLockWithDigestField(t *testing.T) {
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/local-subchart-0.1.0.tgz"))
	defer srv.Stop()
	require.NoError(t, srv.LinkIndices())

	parentPath, _ := setupParentChart(t, srv, &chart.Dependency{
		Name:       "local-subchart",
		Version:    "0.1.0",
		Repository: srv.URL(),
	})

	m, _ := setupDigestTestManager(t, parentPath, srv)
	require.NoError(t, m.Update())

	c, err := loader.LoadDir(parentPath)
	require.NoError(t, err)
	require.NotNil(t, c.Lock)
	require.NotEmpty(t, c.Lock.Dependencies[0].Digest)
}
