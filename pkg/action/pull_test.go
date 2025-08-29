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
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
)

// helm pull testchart --repo=http://127.0.0.1:<port> --version 1.2.3
func TestPull_PrintsSummary_ForHTTPRepo(t *testing.T) {
	t.Parallel()

	// Minimal chart payload; verification is off so a plain byte buffer is fine.
	chartBytes := []byte("dummy-chart-content")
	sum := sha256.Sum256(chartBytes)
	wantDigest := fmt.Sprintf("sha256:%x", sum)

	// Serve a valid index.yaml and the chart archive.
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		// Valid index entry requires both name and version.
		fmt.Fprintf(w, `apiVersion: v1
entries:
  testchart:
    - name: testchart
      version: 1.2.3
      urls:
        - testchart-1.2.3.tgz
      created: "2020-01-01T00:00:00Z"
`)
	})
	mux.HandleFunc("/testchart-1.2.3.tgz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(chartBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	settings := cli.New()
	settings.RepositoryCache = t.TempDir()
	settings.RepositoryConfig = filepath.Join(t.TempDir(), "repositories.yaml")
	settings.ContentCache = t.TempDir()

	cfg := &Configuration{}

	p := NewPull(WithConfig(cfg))
	p.Settings = settings
	p.DestDir = t.TempDir()
	p.RepoURL = srv.URL
	p.Version = "1.2.3"

	out, err := p.Run("testchart")
	require.NoError(t, err, "Pull.Run() should succeed. Output:\n%s", out)

	expectedURL := srv.URL + "/testchart:1.2.3"
	assert.Contains(t, out, "Pulled: "+expectedURL, "expected Pulled summary in output")
	assert.Contains(t, out, "Digest: "+wantDigest, "expected archive digest in output")

	// Ensure the chart file was saved.
	_, statErr := os.Stat(filepath.Join(p.DestDir, "testchart-1.2.3.tgz"))
	require.NoError(t, statErr, "expected chart archive to be saved")
}

// helm pull http://127.0.0.1:<port>/directchart-9.9.9.tgz
func TestPull_PrintsSummary_ForDirectHTTPURL(t *testing.T) {
	t.Parallel()

	chartBytes := []byte("another-dummy-chart")
	sum := sha256.Sum256(chartBytes)
	wantDigest := fmt.Sprintf("sha256:%x", sum)

	mux := http.NewServeMux()
	mux.HandleFunc("/directchart-9.9.9.tar.gz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(chartBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	settings := cli.New()
	settings.RepositoryCache = t.TempDir()
	settings.RepositoryConfig = filepath.Join(t.TempDir(), "repositories.yaml")
	settings.ContentCache = t.TempDir()

	cfg := &Configuration{}

	p := NewPull(WithConfig(cfg))
	p.Settings = settings
	p.DestDir = t.TempDir()

	// Direct HTTP URL (absolute URL). Version is ignored for absolute URLs.
	chartURL := srv.URL + "/directchart-9.9.9.tar.gz"

	out, err := p.Run(chartURL)
	require.NoError(t, err, "Pull.Run() should succeed. Output:\n%s", out)

	// Output should reflect name-version.tgz from the URL.
	expectedURL := srv.URL + "/directchart:9.9.9"
	assert.Contains(t, out, "Pulled: "+expectedURL, "expected Pulled summary in output")
	assert.Contains(t, out, "Digest: "+wantDigest, "expected archive digest in output")

	// Ensure the chart file was saved.
	_, statErr := os.Stat(filepath.Join(p.DestDir, "directchart-9.9.9.tar.gz"))
	require.NoError(t, statErr, "expected chart archive to be saved")
}
