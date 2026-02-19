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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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
	if err != nil {
		t.Fatalf("Problem loading chart repository from %s: %v", repoURL, err)
	}
	repo.CachePath = t.TempDir()

	tempIndexFile, err := os.CreateTemp(t.TempDir(), "test-repo")
	if err != nil {
		t.Fatalf("Failed to create temp index file: %v", err)
	}
	defer os.Remove(tempIndexFile.Name())

	idx, err := repo.DownloadIndexFile()
	if err != nil {
		t.Fatalf("Failed to download index file to %s: %v", idx, err)
	}

	if len(myCustomGetter.repoUrls) != 1 {
		t.Fatalf("Custom Getter.Get should be called once")
	}

	expectedRepoIndexURL := repoURL + "/index.yaml"
	if myCustomGetter.repoUrls[0] != expectedRepoIndexURL {
		t.Fatalf("Custom Getter.Get should be called with %s", expectedRepoIndexURL)
	}
}

func TestConcurrencyDownloadIndex(t *testing.T) {
	srv, err := startLocalServerForTests(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	repo, err := NewChartRepository(&Entry{
		Name: "nginx",
		URL:  srv.URL,
	}, getter.All(&cli.EnvSettings{}))

	if err != nil {
		t.Fatalf("Problem loading chart repository from %s: %v", srv.URL, err)
	}
	repo.CachePath = t.TempDir()

	// initial download index
	idx, err := repo.DownloadIndexFile()
	if err != nil {
		t.Fatalf("Failed to download index file to %s: %v", idx, err)
	}

	indexFName := filepath.Join(repo.CachePath, helmpath.CacheIndexFile(repo.Config.Name))

	var wg sync.WaitGroup

	// Simultaneously start multiple goroutines that:
	// 1) download index.yaml via DownloadIndexFile (write operation),
	// 2) read index.yaml via LoadIndexFile (read operation).
	// This checks for race conditions and ensures correct behavior under concurrent read/write access.
	for range 150 {

		wg.Go(func() {
			idx, err := repo.DownloadIndexFile()
			if err != nil {
				t.Errorf("Failed to download index file to %s: %v", idx, err)
			}
		})

		wg.Go(func() {
			_, err := LoadIndexFile(indexFName)
			if err != nil {
				t.Errorf("Failed to load index file: %v", err)
			}
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
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	chartURL, err := FindChartInRepoURL(
		srv.URL,
		"nginx",
		getter.All(&cli.EnvSettings{}),
		WithInsecureSkipTLSVerify(true),
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if chartURL != "https://charts.helm.sh/stable/nginx-0.2.0.tgz" {
		t.Errorf("%s is not the valid URL", chartURL)
	}

	// If the insecureSkipTLSVerify is false, it will return an error that contains "x509: certificate signed by unknown authority".
	_, err = FindChartInRepoURL(srv.URL, "nginx", getter.All(&cli.EnvSettings{}), WithChartVersion("0.1.0"))
	// Go communicates with the platform and different platforms return different messages. Go itself tests darwin
	// differently for its message. On newer versions of Darwin the message includes the "Acme Co" portion while older
	// versions of Darwin do not. As there are people developing Helm using both old and new versions of Darwin we test
	// for both messages.
	if runtime.GOOS == "darwin" {
		if !strings.Contains(err.Error(), "x509: “Acme Co” certificate is not trusted") && !strings.Contains(err.Error(), "x509: certificate signed by unknown authority") {
			t.Errorf("Expected TLS error for function  FindChartInAuthAndTLSAndPassRepoURL not found, but got a different error (%v)", err)
		}
	} else if !strings.Contains(err.Error(), "x509: certificate signed by unknown authority") {
		t.Errorf("Expected TLS error for function  FindChartInAuthAndTLSAndPassRepoURL not found, but got a different error (%v)", err)
	}
}

func TestFindChartInRepoURL(t *testing.T) {
	srv, err := startLocalServerForTests(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	chartURL, err := FindChartInRepoURL(srv.URL, "nginx", getter.All(&cli.EnvSettings{}))
	if err != nil {
		t.Fatalf("%v", err)
	}
	if chartURL != "https://charts.helm.sh/stable/nginx-0.2.0.tgz" {
		t.Errorf("%s is not the valid URL", chartURL)
	}

	chartURL, err = FindChartInRepoURL(srv.URL, "nginx", getter.All(&cli.EnvSettings{}), WithChartVersion("0.1.0"))
	if err != nil {
		t.Errorf("%s", err)
	}
	if chartURL != "https://charts.helm.sh/stable/nginx-0.1.0.tgz" {
		t.Errorf("%s is not the valid URL", chartURL)
	}
}

func TestErrorFindChartInRepoURL(t *testing.T) {

	g := getter.All(&cli.EnvSettings{
		RepositoryCache: t.TempDir(),
	})

	if _, err := FindChartInRepoURL("http://someserver/something", "nginx", g); err == nil {
		t.Errorf("Expected error for bad chart URL, but did not get any errors")
	} else if !strings.Contains(err.Error(), `looks like "http://someserver/something" is not a valid chart repository or cannot be reached`) {
		t.Errorf("Expected error for bad chart URL, but got a different error (%v)", err)
	}

	srv, err := startLocalServerForTests(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if _, err = FindChartInRepoURL(srv.URL, "nginx1", g); err == nil {
		t.Errorf("Expected error for chart not found, but did not get any errors")
	} else if err.Error() != `chart "nginx1" not found in `+srv.URL+` repository` {
		t.Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}
	if !errors.Is(err, ChartNotFoundError{}) {
		t.Errorf("error is not of correct error type structure")
	}

	if _, err = FindChartInRepoURL(srv.URL, "nginx1", g, WithChartVersion("0.1.0")); err == nil {
		t.Errorf("Expected error for chart not found, but did not get any errors")
	} else if err.Error() != `chart "nginx1" version "0.1.0" not found in `+srv.URL+` repository` {
		t.Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}

	if _, err = FindChartInRepoURL(srv.URL, "chartWithNoURL", g); err == nil {
		t.Errorf("Expected error for no chart URLs available, but did not get any errors")
	} else if err.Error() != `chart "chartWithNoURL" has no downloadable URLs` {
		t.Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}
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
		if err != nil {
			t.Errorf("unexpected error in ResolveReferenceURL(%q, %q): %s", tt.baseURL, tt.refURL, err)
		}
		if chartURL != tt.chartURL {
			t.Errorf("expected ResolveReferenceURL(%q, %q) to equal %q, got %q", tt.baseURL, tt.refURL, tt.chartURL, chartURL)
		}
	}
}
