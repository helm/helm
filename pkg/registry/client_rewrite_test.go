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

package registry

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	"github.com/oras-project/oras-go/v3/registry/remote"
	"github.com/stretchr/testify/require"
)

// startInMemoryRegistry starts an unauthenticated plain-HTTP registry and
// returns its host:port.
func startInMemoryRegistry(t *testing.T) string {
	t.Helper()

	lnCfg := net.ListenConfig{}
	ln, err := lnCfg.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	config := &configuration.Configuration{}
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]any{}}

	reg, err := registry.NewRegistry(t.Context(), config)
	require.NoError(t, err)
	go func() { _ = reg.ListenAndServe() }()
	t.Cleanup(func() { _ = reg.Shutdown(t.Context()) })

	require.Eventually(t, func() bool {
		conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 50*time.Millisecond)

	return addr
}

// TestLocationRewrite_PushPull verifies that a registries.conf location
// rewrite works for every repository operation: the reference handed to
// oras-go must match the rewritten repository, not the alias host.
func TestLocationRewrite_PushPull(t *testing.T) {
	canonicalHost := startInMemoryRegistry(t)
	aliasHost := "charts.example.test"

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		ClientOptPlainHTTP(),
		withRegistriesConfigPath(writeRegistriesConf(t, fmt.Sprintf(
			"[[registry]]\nprefix = %q\nlocation = %q\n", aliasHost, canonicalHost,
		))),
	)
	require.NoError(t, err)

	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	require.NoError(t, err)
	meta, err := extractChartMeta(chartData)
	require.NoError(t, err)

	aliasRepo := fmt.Sprintf("%s/testrepo/%s", aliasHost, meta.Name)
	aliasRef := aliasRepo + ":" + meta.Version

	pushResult, err := c.Push(chartData, aliasRef)
	require.NoError(t, err, "push through the alias")

	desc, err := c.Resolve(aliasRef)
	require.NoError(t, err, "resolve through the alias")
	require.Equal(t, pushResult.Manifest.Digest, desc.Digest.String())

	tags, err := c.Tags(aliasRepo)
	require.NoError(t, err, "list tags through the alias")
	require.Equal(t, []string{meta.Version}, tags)

	pullResult, err := c.Pull(aliasRef)
	require.NoError(t, err, "pull by tag through the alias")
	require.Equal(t, chartData, pullResult.Chart.Data)

	_, err = c.Pull(aliasRepo + "@" + pushResult.Manifest.Digest)
	require.NoError(t, err, "pull by digest through the alias")

	// The chart landed on the canonical host.
	_, err = c.Resolve(fmt.Sprintf("%s/testrepo/%s:%s", canonicalHost, meta.Name, meta.Version))
	require.NoError(t, err, "chart is stored on the canonical host")
}

func TestReferenceForRepository(t *testing.T) {
	repo, err := remote.NewRepository("localhost:5000/charts/app")
	require.NoError(t, err)
	digest := "sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"tag", "example.test/charts/app:1.2.3", "localhost:5000/charts/app:1.2.3"},
		{"tag with plus", "example.test/charts/app:1.2.3+build", "localhost:5000/charts/app:1.2.3_build"},
		{"digest", "example.test/charts/app@" + digest, "localhost:5000/charts/app@" + digest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := newReference(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, ref.forRepository(repo))
		})
	}
}
