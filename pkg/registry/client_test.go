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
	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/policy"
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
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
	)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

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
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

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

func TestNamespacedStore_HierarchicalLookup(t *testing.T) {
	t.Parallel()

	inner := &memCredStore{creds: map[string]credentials.Credential{}}
	ctx := context.Background()

	// Store a credential under "localhost:5000/org".
	_ = inner.Put(ctx, "localhost:5000/org", credentials.Credential{Username: "orguser"})

	// Lookup for a deeper path should find it.
	ns := &namespacedStore{inner: inner, repository: "org/repo"}
	cred, err := ns.Get(ctx, "localhost:5000")
	require.NoError(t, err)
	require.Equal(t, "orguser", cred.Username)

	// Lookup for an unrelated path should NOT find it.
	ns2 := &namespacedStore{inner: inner, repository: "other/repo"}
	cred, err = ns2.Get(ctx, "localhost:5000")
	require.NoError(t, err)
	require.Equal(t, "", cred.Username)
}

// memCredStore is a simple in-memory credentials.Store for testing.
type memCredStore struct {
	creds map[string]credentials.Credential
}

func (m *memCredStore) Get(_ context.Context, serverAddress string) (credentials.Credential, error) {
	return m.creds[serverAddress], nil
}

func (m *memCredStore) Put(_ context.Context, serverAddress string, cred credentials.Credential) error {
	m.creds[serverAddress] = cred
	return nil
}

func (m *memCredStore) Delete(_ context.Context, serverAddress string) error {
	delete(m.creds, serverAddress)
	return nil
}

func TestNewClient_WithDenyAllPolicy(t *testing.T) {
	t.Parallel()

	denyPolicy := policy.NewRejectAllPolicy()
	evaluator, err := policy.NewEvaluator(denyPolicy)
	require.NoError(t, err)

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
		ClientOptPolicyEvaluator(evaluator),
	)
	require.NoError(t, err)
	require.Same(t, evaluator, c.policyEvaluator)
}

func TestNewClient_WithConfigOptions(t *testing.T) {
	t.Parallel()

	credFile := filepath.Join(t.TempDir(), "config.json")
	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(credFile),
		ClientOptConfigOptions(ConfigOptions{
			RegistriesConfigPath: "/nonexistent/registries.conf",
		}),
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
		ClientOptConfigOptions(ConfigOptions{RegistriesConfigPath: registriesConf}),
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
