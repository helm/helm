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

package repotest

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/repo/v1"
)

// Young'n, in these here parts, we test our tests.

func TestServer(t *testing.T) {
	ensure.HelmHome(t)

	rootDir := t.TempDir()

	srv := newServer(t, rootDir)
	defer srv.Stop()

	c, err := srv.CopyCharts("testdata/*.tgz")
	require.NoError(t, err)
	require.Len(t, c, 1)
	assert.Equal(t, "examplechart-0.1.0.tgz", filepath.Base(c[0]))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL()+"/examplechart-0.1.0.tgz", http.NoBody)
	require.NoError(t, err)

	client := http.DefaultClient
	res, err := client.Do(req)
	require.NoError(t, err)

	res.Body.Close()
	assert.GreaterOrEqual(t, res.ContentLength, int64(500))

	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL()+"/index.yaml", http.NoBody)
	require.NoError(t, err)

	res, err = client.Do(req)
	require.NoError(t, err)

	data, err := io.ReadAll(res.Body)
	res.Body.Close()
	require.NoError(t, err)

	m := repo.NewIndexFile()
	require.NoError(t, yaml.Unmarshal(data, m))

	require.Len(t, m.Entries, 1)

	expect := "examplechart"
	assert.True(t, m.Has(expect, "0.1.0"), "missing %q", expect)

	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL()+"/index.yaml-nosuchthing", http.NoBody)
	require.NoError(t, err)
	res, err = client.Do(req)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestNewTempServer(t *testing.T) {
	ensure.HelmHome(t)

	type testCase struct {
		options []ServerOption
	}

	testCases := map[string]testCase{
		"plainhttp": {
			options: []ServerOption{
				WithChartSourceGlob("testdata/examplechart-0.1.0.tgz"),
			},
		},
		"tls": {
			options: []ServerOption{
				WithChartSourceGlob("testdata/examplechart-0.1.0.tgz"),
				WithTLSConfig(MakeTestTLSConfig(t, "../../../../testdata")),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			srv := NewTempServer(
				t,
				tc.options...,
			)
			defer srv.Stop()

			require.NotEmpty(t, srv.srv.URL, "unstarted server")

			client := srv.Client()

			{
				req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, srv.URL()+"/repositories.yaml", http.NoBody)
				require.NoError(t, err)
				res, err := client.Do(req)
				require.NoError(t, err)
				res.Body.Close()
				assert.Equal(t, http.StatusOK, res.StatusCode)
			}
			{
				req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, srv.URL()+"/examplechart-0.1.0.tgz", http.NoBody)
				require.NoError(t, err)
				res, err := client.Do(req)
				require.NoError(t, err)
				res.Body.Close()
				assert.Equal(t, http.StatusOK, res.StatusCode)
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL()+"/examplechart-0.1.0.tgz", http.NoBody)
			require.NoError(t, err)
			res, err := client.Do(req)
			require.NoError(t, err)
			res.Body.Close()
			assert.GreaterOrEqual(t, res.ContentLength, int64(500))
			req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL()+"/index.yaml", http.NoBody)
			require.NoError(t, err)
			res, err = client.Do(req)
			require.NoError(t, err)
			data, err := io.ReadAll(res.Body)
			res.Body.Close()
			require.NoError(t, err)
			m := repo.NewIndexFile()
			require.NoError(t, yaml.Unmarshal(data, m))
			require.Len(t, m.Entries, 1)
			expect := "examplechart"
			assert.True(t, m.Has(expect, "0.1.0"), "missing %q", expect)
			req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL()+"/index.yaml-nosuchthing", http.NoBody)
			require.NoError(t, err)
			res, err = client.Do(req)
			require.NoError(t, err)
			res.Body.Close()
			require.Equal(t, http.StatusNotFound, res.StatusCode)
		})
	}
}

func TestNewTempServer_TLS(t *testing.T) {
	ensure.HelmHome(t)

	srv := NewTempServer(
		t,
		WithChartSourceGlob("testdata/examplechart-0.1.0.tgz"),
		WithTLSConfig(MakeTestTLSConfig(t, "../../../../testdata")),
	)
	defer srv.Stop()

	require.True(t, strings.HasPrefix(srv.URL(), "https://"), "non-TLS server")
}
