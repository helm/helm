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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content/memory"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	remoteconfig "github.com/oras-project/oras-go/v3/registry/remote/config"
	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/policy"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
	"github.com/stretchr/testify/require"
)

// Inspired by oras test
// https://github.com/oras-project/oras-go/blob/05a2b09cbf2eab1df691411884dc4df741ec56ab/content_test.go#L1802
func TestTagManifestTransformsReferences(t *testing.T) {
	memStore := memory.New()
	client := &Client{out: io.Discard}
	ctx := t.Context()

	refWithPlus := "test-registry.io/charts/test:1.0.0+metadata"
	expectedRef := "test-registry.io/charts/test:1.0.0_metadata" // + becomes _

	configDesc := ocispec.Descriptor{MediaType: ConfigMediaType, Digest: "sha256:config", Size: 100}
	layers := []ocispec.Descriptor{{MediaType: ChartLayerMediaType, Digest: "sha256:layer", Size: 200}}

	parsedRef, err := newReference(refWithPlus)
	require.NoError(t, err)

	desc, err := client.tagManifest(ctx, memStore, configDesc, layers, nil, parsedRef)
	require.NoError(t, err)

	transformedDesc, err := memStore.Resolve(ctx, expectedRef)
	require.NoError(t, err, "Should find the reference with _ instead of +")
	require.Equal(t, desc.Digest, transformedDesc.Digest)

	_, err = memStore.Resolve(ctx, refWithPlus)
	require.Error(t, err, "Should NOT find the reference with the original +")
}

// Verifies that the authorizer is set on a new client and Login succeeds against a reachable registry.
func TestLogin_AuthorizerSetAndSucceeds(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			// Accept either HEAD or GET
			w.WriteHeader(http.StatusOK)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	require.NoError(t, err, "NewClient error")

	if c.authorizer == nil {
		t.Fatal("expected authorizer to be set")
	}

	// Call Login with plain HTTP against our test server
	if err := c.Login(host, LoginOptPlainText(true), LoginOptBasicAuth("u", "p")); err != nil {
		t.Fatalf("Login error: %v", err)
	}
}

// Verifies that Login returns an error when the registry is unreachable.
func TestLogin_FailsWhenUnreachable(t *testing.T) {
	t.Parallel()

	// Start and immediately close, so connections will fail
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	host := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	require.NoError(t, err, "NewClient error")

	// Invoke Login, expect an error since the server is closed
	if err := c.Login(host, LoginOptPlainText(true), LoginOptBasicAuth("u", "p")); err == nil {
		t.Error("expected Login to fail when server is unreachable")
	}
}

func TestLogin_NamespacedAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	registryHost := strings.TrimPrefix(srv.URL, "http://")
	namespacedHost := registryHost + "/myrepo"

	// Pre-create a config.json with a sentinel auths entry so that
	// IsAuthConfigured() returns true and the credentials package does not
	// detect a native platform store (e.g., osxkeychain on macOS). This
	// ensures the credential is written into the plaintext config file
	// where we can inspect the storage key directly.
	credFile := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(credFile, []byte(`{"auths":{"_sentinel_":{}}}`), 0o600)
	require.NoError(t, err)

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	require.NoError(t, err)

	err = c.Login(namespacedHost, LoginOptPlainText(true), LoginOptBasicAuth("u", "p"))
	require.NoError(t, err)

	ctx := context.Background()

	// Credential lookup by namespaced key succeeds.
	cred, err := c.credentialsStore.Get(ctx, namespacedHost)
	require.NoError(t, err)
	require.Equal(t, "u", cred.Username)

	// Verify that the credential is stored on disk under the namespaced key,
	// and NOT under the hostname-only key. We inspect the JSON file directly
	// because the FileStore's Get() falls back to a hostname-based lookup,
	// which would mask whether the storage key itself is hostname-only or
	// namespaced.
	data, err := os.ReadFile(credFile)
	require.NoError(t, err)
	var parsed struct {
		Auths map[string]any `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Contains(t, parsed.Auths, namespacedHost, "credential should be stored under namespaced key")
	require.NotContains(t, parsed.Auths, registryHost, "credential should not be stored under hostname-only key")
}

func TestClientBuilder_CredentialLookup(t *testing.T) {
	t.Parallel()

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	require.NoError(t, err)

	ctx := context.Background()
	const host = "creds.example.test"
	require.NoError(t, c.credentialsStore.Put(ctx, host+"/org", credentials.Credential{Username: "orguser", Password: "p"}))
	require.NoError(t, c.credentialsStore.Put(ctx, "https://index.docker.io/v1/", credentials.Credential{Username: "hubuser", Password: "p"}))

	tests := []struct {
		name     string
		resource properties.Resource
		want     string
	}{
		{"namespaced credential applies to a deeper path", properties.Resource{Registry: host, Path: "org/repo"}, "orguser"},
		{"docker hub maps to its server address", properties.Resource{Registry: "docker.io", Path: "library/nginx"}, "hubuser"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authClient, err := c.builder.Build(properties.NewRegistryFromReference(properties.Reference{
				Registry:   tt.resource.Registry,
				Repository: tt.resource.Path,
			}))
			require.NoError(t, err)
			cred, err := authClient.CredentialFunc(ctx, tt.resource)
			require.NoError(t, err)
			require.Equal(t, tt.want, cred.Username)
		})
	}
}

func TestNewClient_WithRegistriesConfigPath(t *testing.T) {
	t.Parallel()

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
		withRegistriesConfigPath("/nonexistent/registries.conf"),
	)
	require.NoError(t, err) // nonexistent paths are silently skipped
	require.NotNil(t, c)
}

func TestLogin_LocationRewrite(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	canonicalHost := strings.TrimPrefix(srv.URL, "http://")
	aliasHost := "registry.example.test"

	// Write a registries.conf that maps the alias to the canonical host.
	registriesConf := filepath.Join(t.TempDir(), "registries.conf")
	err := os.WriteFile(registriesConf, fmt.Appendf(nil,
		"[[registry]]\nprefix = %q\nlocation = %q\n", aliasHost, canonicalHost,
	), 0o600)
	require.NoError(t, err)

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
		withRegistriesConfigPath(registriesConf),
	)
	require.NoError(t, err)

	// Login via alias; the connection goes to the canonical host (plain HTTP).
	require.NoError(t, c.Login(aliasHost, LoginOptPlainText(true), LoginOptBasicAuth("u", "p")))

	// Credential must be stored under the canonical host, not the alias.
	cred, err := c.credentialsStore.Get(context.Background(), canonicalHost)
	require.NoError(t, err)
	require.Equal(t, "u", cred.Username, "credential should be stored under canonical host")

	aliasCred, err := c.credentialsStore.Get(context.Background(), aliasHost)
	require.NoError(t, err)
	require.Empty(t, aliasCred.Username, "credential must not be stored under alias")
}

// mockRemoteClient records whether Do was called, for use in registryAuthorizer tests.
type mockRemoteClient struct {
	called bool
	inner  http.RoundTripper
}

func (m *mockRemoteClient) Do(req *http.Request) (*http.Response, error) {
	m.called = true
	return m.inner.RoundTrip(req)
}

func TestRegistryAuthorizer_UsedInLegacyPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Return empty tag list for any tags request.
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name":"testchart","tags":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	mock := &mockRemoteClient{inner: http.DefaultTransport}

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
		ClientOptHTTPClient(&http.Client{}), // triggers customHTTPClient=true (legacy path)
		ClientOptRegistryAuthorizer(mock),
		ClientOptPlainHTTP(),
	)
	require.NoError(t, err)

	// Tags calls newRepository → legacy path → should use registryAuthorizer.
	_, err = c.Tags(host + "/testchart")
	require.NoError(t, err)
	require.True(t, mock.called, "registryAuthorizer.Do should have been called in legacy path")
}

func TestLogin_RegistryAuthorizerInLegacyPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		ClientOptHTTPClient(&http.Client{}),
		ClientOptRegistryAuthorizer(&mockRemoteClient{inner: http.DefaultTransport}),
	)
	require.NoError(t, err)

	// remote.Login only accepts an *auth.Client, so Login must not hand it
	// the custom registry authorizer.
	require.NoError(t, c.Login(host, LoginOptPlainText(true), LoginOptBasicAuth("u", "p")))
}

func writeRegistriesConf(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registries.conf")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLogin_BlockedRegistry(t *testing.T) {
	t.Parallel()

	host := "blocked.example.test"
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		withRegistriesConfigPath(writeRegistriesConf(t, fmt.Sprintf("[[registry]]\nprefix = %q\nblocked = true\n", host))),
	)
	require.NoError(t, err)

	err = c.Login(host, LoginOptBasicAuth("u", "p"))
	require.ErrorIs(t, err, remoteconfig.ErrRegistryBlocked)
}

func TestNewRegistry_TokenFlow(t *testing.T) {
	t.Parallel()

	host := "tokenflow.example.test"
	tests := []struct {
		name        string
		tokenFlow   string
		wantFetcher bool
		wantErr     bool
	}{
		{name: "default", tokenFlow: "", wantFetcher: false},
		{name: "distribution", tokenFlow: "distribution", wantFetcher: true},
		{name: "invalid", tokenFlow: "bogus", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conf := fmt.Sprintf("[[registry]]\nprefix = %q\n", host)
			if tt.tokenFlow != "" {
				conf += fmt.Sprintf("token-flow = %q\n", tt.tokenFlow)
			}
			c, err := NewClient(
				ClientOptWriter(io.Discard),
				ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
				withRegistriesConfigPath(writeRegistriesConf(t, conf)),
			)
			require.NoError(t, err)

			reg, err := c.newRegistry(host)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			authClient, ok := reg.Client.(*auth.Client)
			require.True(t, ok)
			require.Equal(t, tt.wantFetcher, authClient.TokenFetcher != nil)
		})
	}
}

func TestNewRepository_SignatureVerificationEvaluator(t *testing.T) {
	t.Parallel()

	newSigClient := func(t *testing.T, options ...ClientOption) *Client {
		t.Helper()
		options = append([]ClientOption{
			ClientOptWriter(io.Discard),
			ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
			ClientOptSignatureVerification(true),
		}, options...)
		c, err := NewClient(options...)
		require.NoError(t, err)

		// Replace the host's config stack with a policy and a registries.d
		// lookaside so a signature verifier is built for every scope.
		c.configs = &remoteconfig.Configs{
			PolicyConfig: policy.NewInsecureAcceptAnythingPolicy(),
			RegistriesDConfig: &remoteconfig.RegistriesDConfig{
				DefaultDocker: &remoteconfig.RegistriesDDockerConfig{Lookaside: "file://" + t.TempDir()},
			},
		}
		return c
	}

	t.Run("config-derived evaluator is replaced", func(t *testing.T) {
		t.Parallel()
		c := newSigClient(t)
		configEval, err := c.configs.PolicyEvaluator()
		require.NoError(t, err)
		c.policyEvaluator = configEval
		c.builder.PolicyEvaluator = configEval

		repo, err := c.newRepository("example.test/charts/mychart:1.0.0")
		require.NoError(t, err)
		require.NotNil(t, repo.Registry.Policy)
		require.NotSame(t, configEval, repo.Registry.Policy, "signature-scoped evaluator should be installed")
	})
}

func TestLogin_PathScopedLocationRewrite(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	canonicalHost := strings.TrimPrefix(srv.URL, "http://")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		withRegistriesConfigPath(writeRegistriesConf(t, fmt.Sprintf(
			"[[registry]]\nprefix = %q\nlocation = %q\n", "alias.example.test/team", canonicalHost+"/org",
		))),
	)
	require.NoError(t, err)

	require.NoError(t, c.Login("alias.example.test/team", LoginOptPlainText(true), LoginOptBasicAuth("u", "p")))

	// Credential must be stored under the rewritten namespace.
	cred, err := c.credentialsStore.Get(context.Background(), canonicalHost+"/org")
	require.NoError(t, err)
	require.Equal(t, "u", cred.Username)

	// Logout with the same alias path must remove the rewritten credential.
	require.NoError(t, c.Logout("alias.example.test/team"))
	cred, err = c.credentialsStore.Get(context.Background(), canonicalHost+"/org")
	require.NoError(t, err)
	require.Equal(t, credentials.EmptyCredential, cred)
}

func TestLogin_PathScopedBlockedRegistry(t *testing.T) {
	t.Parallel()

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		withRegistriesConfigPath(writeRegistriesConf(t, "[[registry]]\nprefix = \"blocked.example.test/team\"\nblocked = true\n")),
	)
	require.NoError(t, err)

	err = c.Login("blocked.example.test/team", LoginOptBasicAuth("u", "p"))
	require.ErrorIs(t, err, remoteconfig.ErrRegistryBlocked)
}

func TestNewClient_DisableCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		enableCache bool
		wantCache   bool
	}{
		{name: "enabled", enableCache: true, wantCache: true},
		{name: "disabled", enableCache: false, wantCache: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := NewClient(
				ClientOptWriter(io.Discard),
				ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
				ClientOptEnableCache(tt.enableCache),
			)
			require.NoError(t, err)

			authClient, err := c.builder.Build(properties.NewRegistryFromReference(properties.Reference{
				Registry:   "cache.example.test",
				Repository: "charts/mychart",
			}))
			require.NoError(t, err)
			require.Equal(t, tt.wantCache, authClient.Cache != nil)
		})
	}
}

func TestNewClient_InvalidConfigDeferred(t *testing.T) {
	t.Parallel()

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		withRegistriesConfigPath(writeRegistriesConf(t, "[[registry]\nnot valid toml")),
	)
	require.NoError(t, err, "an unparsable config file must not fail NewClient")

	_, err = c.Tags("example.test/charts/mychart")
	require.ErrorContains(t, err, "failed to load registry configurations")

	err = c.Login("example.test", LoginOptBasicAuth("u", "p"))
	require.ErrorContains(t, err, "failed to load registry configurations")

	err = c.Logout("example.test")
	require.ErrorContains(t, err, "failed to load registry configurations")
}

func TestNewClient_PolicyEnforcement(t *testing.T) {
	t.Parallel()

	policyPath := filepath.Join(t.TempDir(), "policy.json")
	require.NoError(t, os.WriteFile(policyPath, []byte(`{"default":[{"type":"reject"}]}`), 0o600))

	tests := []struct {
		name          string
		options       []ClientOption
		wantEvaluator bool
	}{
		{name: "not enforced by default", wantEvaluator: false},
		{name: "enforced when enabled", options: []ClientOption{ClientOptPolicyEnforcement(true)}, wantEvaluator: true},
		{name: "enforced with signature verification", options: []ClientOption{ClientOptSignatureVerification(true)}, wantEvaluator: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			options := append([]ClientOption{
				ClientOptWriter(io.Discard),
				ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
				withPolicyConfigPath(policyPath),
			}, tt.options...)
			c, err := NewClient(options...)
			require.NoError(t, err)

			require.Equal(t, tt.wantEvaluator, c.policyEvaluator != nil)
			require.Equal(t, tt.wantEvaluator, c.builder.PolicyEvaluator != nil)
		})
	}
}

func TestClientOptTLSClientConfig(t *testing.T) {
	t.Parallel()

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		ClientOptTLSClientConfig("client.crt", "client.key", "ca.crt"),
		ClientOptInsecureSkipVerifyTLS(true),
	)
	require.NoError(t, err)
	require.False(t, c.customHTTPClient, "TLS options must keep the config-driven path")

	props := properties.NewRegistryFromReference(properties.Reference{Registry: "tls.example.test"})
	c.applyOverrides(props)
	require.Equal(t, "client.crt", props.Transport.Cert)
	require.Equal(t, "client.key", props.Transport.Key)
	require.Contains(t, props.Transport.CACerts, "ca.crt")
	require.True(t, props.Transport.Insecure)
}

func TestClientOptTLSClientConfig_CertWithoutKey(t *testing.T) {
	t.Parallel()

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		ClientOptTLSClientConfig("client.crt", "", ""),
	)
	require.NoError(t, err)
	require.Empty(t, c.certFile)
	require.Empty(t, c.keyFile)
}
