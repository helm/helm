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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/registry"
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
	srv, err := startLocalServerForTests(t, nil)
	if err != nil {
		t.Fatal(err)
	}
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
			_, err = w.Write(fileBytes)
			require.NoError(t, err)
		})
	}

	return httptest.NewServer(handler), nil
}
