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
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
)

const (
	testRepository = "testdata/repository"
	testURL        = "http://example-charts.com"
)

func TestLoadChartRepository(t *testing.T) {
	r, err := NewChartRepository(&Entry{
		Name: testRepository,
		URL:  testURL,
	}, getter.All(&cli.EnvSettings{}))
	if err != nil {
		t.Errorf("Problem creating chart repository from %s: %v", testRepository, err)
	}

	if err := r.Load(); err != nil {
		t.Errorf("Problem loading chart repository from %s: %v", testRepository, err)
	}

	paths := []string{
		filepath.Join(testRepository, "frobnitz-1.2.3.tgz"),
		filepath.Join(testRepository, "sprocket-1.1.0.tgz"),
		filepath.Join(testRepository, "sprocket-1.2.0.tgz"),
		filepath.Join(testRepository, "universe/zarthal-1.0.0.tgz"),
	}

	if r.Config.Name != testRepository {
		t.Errorf("Expected %s as Name but got %s", testRepository, r.Config.Name)
	}

	if !reflect.DeepEqual(r.ChartPaths, paths) {
		t.Errorf("Expected %#v but got %#v\n", paths, r.ChartPaths)
	}

	if r.Config.URL != testURL {
		t.Errorf("Expected url for chart repository to be %s but got %s", testURL, r.Config.URL)
	}
}

func TestIndex(t *testing.T) {
	r, err := NewChartRepository(&Entry{
		Name: testRepository,
		URL:  testURL,
	}, getter.All(&cli.EnvSettings{}))
	if err != nil {
		t.Errorf("Problem creating chart repository from %s: %v", testRepository, err)
	}

	if err := r.Load(); err != nil {
		t.Errorf("Problem loading chart repository from %s: %v", testRepository, err)
	}

	err = r.Index()
	if err != nil {
		t.Errorf("Error performing index: %v\n", err)
	}

	tempIndexPath := filepath.Join(testRepository, indexPath)
	actual, err := LoadIndexFile(tempIndexPath)
	defer os.Remove(tempIndexPath) // clean up
	if err != nil {
		t.Errorf("Error loading index file %v", err)
	}
	verifyIndex(t, actual)

	// Re-index and test again.
	err = r.Index()
	if err != nil {
		t.Errorf("Error performing re-index: %s\n", err)
	}
	second, err := LoadIndexFile(tempIndexPath)
	if err != nil {
		t.Errorf("Error re-loading index file %v", err)
	}
	verifyIndex(t, second)
}

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

	tempIndexFile, err := os.CreateTemp("", "test-repo")
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

func verifyIndex(t *testing.T, actual *IndexFile) {
	var empty time.Time
	if actual.Generated.Equal(empty) {
		t.Errorf("Generated should be greater than 0: %s", actual.Generated)
	}

	if actual.APIVersion != APIVersionV1 {
		t.Error("Expected v1 API")
	}

	entries := actual.Entries
	if numEntries := len(entries); numEntries != 3 {
		t.Errorf("Expected 3 charts to be listed in index file but got %v", numEntries)
	}

	expects := map[string]ChartVersions{
		"frobnitz": {
			{
				Metadata: &chart.Metadata{
					Name:    "frobnitz",
					Version: "1.2.3",
				},
			},
		},
		"sprocket": {
			{
				Metadata: &chart.Metadata{
					Name:    "sprocket",
					Version: "1.2.0",
				},
			},
			{
				Metadata: &chart.Metadata{
					Name:    "sprocket",
					Version: "1.1.0",
				},
			},
		},
		"zarthal": {
			{
				Metadata: &chart.Metadata{
					Name:    "zarthal",
					Version: "1.0.0",
				},
			},
		},
	}

	for name, versions := range expects {
		got, ok := entries[name]
		if !ok {
			t.Errorf("Could not find %q entry", name)
			continue
		}
		if len(versions) != len(got) {
			t.Errorf("Expected %d versions, got %d", len(versions), len(got))
			continue
		}
		for i, e := range versions {
			g := got[i]
			if e.Name != g.Name {
				t.Errorf("Expected %q, got %q", e.Name, g.Name)
			}
			if e.Version != g.Version {
				t.Errorf("Expected %q, got %q", e.Version, g.Version)
			}
			if len(g.Keywords) != 3 {
				t.Error("Expected 3 keywords.")
			}
			if len(g.Maintainers) != 2 {
				t.Error("Expected 2 maintainers.")
			}
			if g.Created.Equal(empty) {
				t.Error("Expected created to be non-empty")
			}
			if g.Description == "" {
				t.Error("Expected description to be non-empty")
			}
			if g.Home == "" {
				t.Error("Expected home to be non-empty")
			}
			if g.Digest == "" {
				t.Error("Expected digest to be non-empty")
			}
			if len(g.URLs) != 1 {
				t.Error("Expected exactly 1 URL")
			}
		}
	}
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

	chartURL, err := FindChartInAuthAndTLSAndPassRepoURL(srv.URL, "", "", "nginx", "", "", "", "", true, false, getter.All(&cli.EnvSettings{}))
	if err != nil {
		t.Fatalf("%v", err)
	}
	if chartURL != "https://charts.helm.sh/stable/nginx-0.2.0.tgz" {
		t.Errorf("%s is not the valid URL", chartURL)
	}

	// If the insecureSkipTLsverify is false, it will return an error that contains "x509: certificate signed by unknown authority".
	_, err = FindChartInAuthAndTLSAndPassRepoURL(srv.URL, "", "", "nginx", "0.1.0", "", "", "", false, false, getter.All(&cli.EnvSettings{}))
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

	chartURL, err := FindChartInRepoURL(srv.URL, "nginx", "", "", "", "", getter.All(&cli.EnvSettings{}))
	if err != nil {
		t.Fatalf("%v", err)
	}
	if chartURL != "https://charts.helm.sh/stable/nginx-0.2.0.tgz" {
		t.Errorf("%s is not the valid URL", chartURL)
	}

	chartURL, err = FindChartInRepoURL(srv.URL, "nginx", "0.1.0", "", "", "", getter.All(&cli.EnvSettings{}))
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

	if _, err := FindChartInRepoURL("http://someserver/something", "nginx", "", "", "", "", g); err == nil {
		t.Errorf("Expected error for bad chart URL, but did not get any errors")
	} else if !strings.Contains(err.Error(), `looks like "http://someserver/something" is not a valid chart repository or cannot be reached`) {
		t.Errorf("Expected error for bad chart URL, but got a different error (%v)", err)
	}

	srv, err := startLocalServerForTests(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if _, err = FindChartInRepoURL(srv.URL, "nginx1", "", "", "", "", g); err == nil {
		t.Errorf("Expected error for chart not found, but did not get any errors")
	} else if err.Error() != `chart "nginx1" not found in `+srv.URL+` repository` {
		t.Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}

	if _, err = FindChartInRepoURL(srv.URL, "nginx1", "0.1.0", "", "", "", g); err == nil {
		t.Errorf("Expected error for chart not found, but did not get any errors")
	} else if err.Error() != `chart "nginx1" version "0.1.0" not found in `+srv.URL+` repository` {
		t.Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}

	if _, err = FindChartInRepoURL(srv.URL, "chartWithNoURL", "", "", "", "", g); err == nil {
		t.Errorf("Expected error for no chart URLs available, but did not get any errors")
	} else if err.Error() != `chart "chartWithNoURL" has no downloadable URLs` {
		t.Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}
}

func TestResolveReferenceURL(t *testing.T) {
	for _, tt := range []struct {
		baseURL, refURL, chartURL string
	}{
		{"http://localhost:8123/charts/", "nginx-0.2.0.tgz", "http://localhost:8123/charts/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts-with-no-trailing-slash", "nginx-0.2.0.tgz", "http://localhost:8123/charts-with-no-trailing-slash/nginx-0.2.0.tgz"},
		{"http://localhost:8123", "https://charts.helm.sh/stable/nginx-0.2.0.tgz", "https://charts.helm.sh/stable/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts%2fwith%2fescaped%2fslash", "nginx-0.2.0.tgz", "http://localhost:8123/charts%2fwith%2fescaped%2fslash/nginx-0.2.0.tgz"},
		{"http://localhost:8123/charts?with=queryparameter", "nginx-0.2.0.tgz", "http://localhost:8123/charts/nginx-0.2.0.tgz?with=queryparameter"},
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
