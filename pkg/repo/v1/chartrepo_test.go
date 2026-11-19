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

package repo

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

type CustomGetter struct {
	repoUrls []string
}

func (g *CustomGetter) Get(href string, _ ...getter.Option) (*bytes.Buffer, error) {
	index := &IndexFile{
		APIVersion: "v1",
		Generated:  time.Now(),
	}
	indexBytes, err := yaml.Marshal(index)
	if err != nil {
		return nil, err
	}
	g.repoUrls = append(g.repoUrls, href)
	return bytes.NewBuffer(indexBytes), nil
}

func TestIndexCustomSchemeDownload(t *testing.T) {
	repoName := "gcs-repo"
	repoURL := "gs://some-gcs-bucket"
	myCustomGetter := &CustomGetter{}
	customGetterConstructor := func(_ ...getter.Option) (getter.Getter, error) {
		return myCustomGetter, nil
	}
	providers := getter.Providers{{
		Schemes: []string{"gs"},
		New:     customGetterConstructor,
	}}
	repo, err := NewChartRepository(&Entry{
		Name: repoName,
		URL:  repoURL,
	}, providers)
	require.NoErrorf(t, err, "Problem loading chart repository from %s", repoURL)
	repo.CachePath = t.TempDir()

	tempIndexFile, err := os.CreateTemp(t.TempDir(), "test-repo")
	require.NoErrorf(t, err, "Failed to create temp index file")
	defer os.Remove(tempIndexFile.Name())

	idx, err := repo.DownloadIndexFile()
	require.NoErrorf(t, err, "Failed to download index file to %s", idx)

	require.Len(t, myCustomGetter.repoUrls, 1, "Custom Getter.Get should be called once")

	expectedRepoIndexURL := repoURL + "/index.yaml"
	require.Equalf(t, expectedRepoIndexURL, myCustomGetter.repoUrls[0], "Custom Getter.Get should be called with %s", expectedRepoIndexURL)
}

func TestConcurrencyDownloadIndex(t *testing.T) {
	srv, err := startLocalServerForTests(nil)
	require.NoError(t, err)
	defer srv.Close()

	repo, err := NewChartRepository(&Entry{
		Name: "nginx",
		URL:  srv.URL,
	}, getter.All(&cli.EnvSettings{}))

	require.NoErrorf(t, err, "Problem loading chart repository from %s", srv.URL)
	repo.CachePath = t.TempDir()

	// initial download index
	idx, err := repo.DownloadIndexFile()
	require.NoErrorf(t, err, "Failed to download index file to %s", idx)

	indexFName := filepath.Join(repo.CachePath, helmpath.CacheIndexFile(repo.Config.Name))

	var wg sync.WaitGroup

	// Simultaneously start multiple goroutines that:
	// 1) download index.yaml via DownloadIndexFile (write operation),
	// 2) read index.yaml via LoadIndexFile (read operation).
	// This checks for race conditions and ensures correct behavior under concurrent read/write access.
	for range 150 {
		wg.Go(func() {
			idx, err := repo.DownloadIndexFile()
			assert.NoErrorf(t, err, "Failed to download index file to %s", idx)
		})

		wg.Go(func() {
			_, err := LoadIndexFile(indexFName)
			assert.NoErrorf(t, err, "Failed to load index file")
		})
	}
	wg.Wait()
}

// startLocalServerForTests Start the local helm server
func startLocalServerForTests(handler http.Handler) (*httptest.Server, error) {
	if handler == nil {
		fileBytes, err := os.ReadFile("testdata/local-index.yaml")
		if err != nil {
			return nil, err
		}
		handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write(fileBytes)
		})
	}

	return httptest.NewServer(handler), nil
}

// startLocalTLSServerForTests Start the local helm server with TLS
func startLocalTLSServerForTests(handler http.Handler) (*httptest.Server, error) {
	if handler == nil {
		fileBytes, err := os.ReadFile("testdata/local-index.yaml")
		if err != nil {
			return nil, err
		}
		handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write(fileBytes)
		})
	}

	return httptest.NewTLSServer(handler), nil
}

func TestFindChartInAuthAndTLSAndPassRepoURL(t *testing.T) {
	srv, err := startLocalTLSServerForTests(nil)
	require.NoError(t, err)
	defer srv.Close()

	chartURL, err := FindChartInRepoURL(
		srv.URL,
		"nginx",
		getter.All(&cli.EnvSettings{}),
		WithInsecureSkipTLSVerify(true),
	)
	require.NoError(t, err)
	assert.Equalf(t, "https://charts.helm.sh/stable/nginx-0.2.0.tgz", chartURL, "%s is not the valid URL", chartURL)

	// If the insecureSkipTLSVerify is false, it will return an error that contains "x509: certificate signed by unknown authority".
	_, err = FindChartInRepoURL(srv.URL, "nginx", getter.All(&cli.EnvSettings{}), WithChartVersion("0.1.0"))
	// Go communicates with the platform and different platforms return different messages. Go itself tests darwin
	// differently for its message. On newer versions of Darwin the message includes the "Acme Co" portion while older
	// versions of Darwin do not. As there are people developing Helm using both old and new versions of Darwin we test
	// for both messages.
	if runtime.GOOS == "darwin" {
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "x509: “Acme Co” certificate is not trusted") || strings.Contains(err.Error(), "x509: certificate signed by unknown authority"), "Expected TLS error for function  FindChartInAuthAndTLSAndPassRepoURL not found, but got a different error (%v)", err)
	} else {
		assert.ErrorContainsf(t, err, "x509: certificate signed by unknown authority", "Expected TLS error for function  FindChartInAuthAndTLSAndPassRepoURL not found, but got a different error")
	}
}

func TestFindChartInRepoURL(t *testing.T) {
	srv, err := startLocalServerForTests(nil)
	require.NoError(t, err)
	defer srv.Close()

	chartURL, err := FindChartInRepoURL(srv.URL, "nginx", getter.All(&cli.EnvSettings{}))
	require.NoError(t, err)
	assert.Equalf(t, "https://charts.helm.sh/stable/nginx-0.2.0.tgz", chartURL, "%s is not the valid URL", chartURL)

	chartURL, err = FindChartInRepoURL(srv.URL, "nginx", getter.All(&cli.EnvSettings{}), WithChartVersion("0.1.0"))
	require.NoError(t, err)
	assert.Equalf(t, "https://charts.helm.sh/stable/nginx-0.1.0.tgz", chartURL, "%s is not the valid URL", chartURL)
}

func TestErrorFindChartInRepoURL(t *testing.T) {
	g := getter.All(&cli.EnvSettings{
		RepositoryCache: t.TempDir(),
	})

	_, err := FindChartInRepoURL("http://someserver/something", "nginx", g)
	require.ErrorContainsf(t, err, `looks like "http://someserver/something" is not a valid chart repository or cannot be reached`, "Expected error for bad chart URL, but got a different error")

	srv, err := startLocalServerForTests(nil)
	require.NoError(t, err)
	defer srv.Close()

	_, err = FindChartInRepoURL(srv.URL, "nginx1", g)
	require.EqualError(t, err, `chart "nginx1" not found in `+srv.URL+` repository`, "Expected error for chart not found, but got a different error")
	require.ErrorIs(t, err, ChartNotFoundError{}, "error is not of correct error type structure")

	_, err = FindChartInRepoURL(srv.URL, "nginx1", g, WithChartVersion("0.1.0"))
	require.EqualError(t, err, `chart "nginx1" version "0.1.0" not found in `+srv.URL+` repository`, "Expected error for chart not found, but got a different error")

	_, err = FindChartInRepoURL(srv.URL, "chartWithNoURL", g)
	assert.EqualError(t, err, `chart "chartWithNoURL" has no downloadable URLs`, "Expected error for chart not found, but got a different error")
}

func TestResolveReferenceURL(t *testing.T) {
	for _, tt := range []struct {
		baseURL, refURL, chartURL string
	}{
		{"http://localhost:8123/", "/nginx-0.2.0.tgz", "http://localhost:8123/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts/", "nginx-0.2.0.tgz", "http://localhost:8123/charts/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts/", "/nginx-0.2.0.tgz", "http://localhost:8123/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts-with-no-trailing-slash", "nginx-0.2.0.tgz", "http://localhost:8123/charts-with-no-trailing-slash/nginx-0.2.0.tgz"},
		{"http://localhost:8123", "https://charts.helm.sh/stable/nginx-0.2.0.tgz", "https://charts.helm.sh/stable/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts%2fwith%2fescaped%2fslash", "nginx-0.2.0.tgz", "http://localhost:8123/charts%2fwith%2fescaped%2fslash/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts%2fwith%2fescaped%2fslash", "/nginx-0.2.0.tgz", "http://localhost:8123/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts?with=queryparameter", "nginx-0.2.0.tgz", "http://localhost:8123/charts/nginx-0.2.0.tgz?with=queryparameter"},
		{"http://localhost:8123/charts?with=queryparameter", "/nginx-0.2.0.tgz", "http://localhost:8123/nginx-0.2.0.tgz?with=queryparameter"},
	} {
		chartURL, err := ResolveReferenceURL(tt.baseURL, tt.refURL)
		require.NoErrorf(t, err, "unexpected error in ResolveReferenceURL(%q, %q)", tt.baseURL, tt.refURL)
		assert.Equalf(t, chartURL, tt.chartURL, "expected ResolveReferenceURL(%q, %q) to equal %q, got %q", tt.baseURL, tt.refURL, tt.chartURL, chartURL)
	}
}
