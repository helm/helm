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

package action

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestNewPull(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewPull(WithConfig(config))

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestPullSetRegistryClient(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewPull(WithConfig(config))

	registryClient := &registry.Client{}
	client.SetRegistryClient(registryClient)
	assert.Equal(t, registryClient, client.cfg.RegistryClient)
}

func TestPullRun_ChartNotFound(t *testing.T) {
	fileBytes, err := os.ReadFile("../repo/v1/testdata/local-index.yaml")
	require.NoError(t, err)
	// The fixture's placeholder digest is not a valid sha256, and --repo pulls
	// check the index digest, so give it a well-formed one to let the pull get
	// as far as the missing archive.
	fileBytes = bytes.ReplaceAll(fileBytes, []byte("sha256:1234567890abcdef"), []byte("sha256:"+strings.Repeat("0", 64)))
	srv, err := startLocalServerForTests(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(fileBytes)
		assert.NoError(t, err)
	}))
	require.NoError(t, err)
	defer srv.Close()

	config := actionConfigFixture(t)
	client := NewPull(WithConfig(config))
	client.Settings = cli.New()
	client.RepoURL = srv.URL

	chartRef := "nginx"
	_, err = client.Run(chartRef)
	require.ErrorContains(t, err, "404 Not Found")
}

func startLocalServerForTests(t *testing.T, handler http.Handler) (*httptest.Server, error) {
	t.Helper()
	if handler == nil {
		fileBytes, err := os.ReadFile("../repo/v1/testdata/local-index.yaml")
		if err != nil {
			return nil, err
		}
		handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write(fileBytes)
			assert.NoError(t, err)
		})
	}

	return httptest.NewServer(handler), nil
}

// tamperedRepoServer serves a chart repository whose index records the digest
// of signtest-0.1.0.tgz while the archive it serves has changed since.
func tamperedRepoServer(t *testing.T) *repotest.Server {
	t.Helper()
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("../downloader/testdata/signtest-0.1.0.tgz"))
	t.Cleanup(srv.Stop)

	served := filepath.Join(srv.Root(), "signtest-0.1.0.tgz")
	original, err := os.ReadFile(served)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(served, append(original, "appended by a rewritten mirror"...), 0o644))
	return srv
}

func TestPullRun_RepoURLRejectsChartNotMatchingIndexDigest(t *testing.T) {
	ensure.HelmHome(t)
	srv := tamperedRepoServer(t)

	config := actionConfigFixture(t)
	client := NewPull(WithConfig(config))
	client.Settings = cli.New()
	client.RepoURL = srv.URL()
	client.Version = "0.1.0"
	client.DestDir = t.TempDir()

	_, err := client.Run("signtest")
	require.ErrorContains(t, err, "does not match the digest recorded for it in the repository index")

	entries, err := os.ReadDir(client.DestDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "rejected chart must not be written to the destination")
}
