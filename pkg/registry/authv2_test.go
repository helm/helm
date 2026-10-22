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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"

	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv2 "oras.land/oras-go/v2/registry/remote/auth"
)

func TestAuthClientFromV2_Fields(t *testing.T) {
	httpClient := &http.Client{}
	header := http.Header{"X-Test": {"value"}}
	v2 := authv2.Client{
		Client:   httpClient,
		Header:   header,
		ClientID: "helm-test",
	}

	got := authClientFromV2(v2)

	assert.Same(t, httpClient, got.Client)
	assert.Equal(t, header, got.Header)
	assert.Equal(t, "helm-test", got.ClientID)
	assert.Nil(t, got.CredentialFunc)
	assert.Nil(t, got.Cache)
	require.NotNil(t, got.TokenFetcher)

	// The converted client owns its header.
	header.Set("X-Test", "changed")
	assert.Equal(t, "value", got.Header.Get("X-Test"))
}

func TestAuthClientFromV2_Credential(t *testing.T) {
	var gotHost string
	v2 := authv2.Client{
		Credential: func(_ context.Context, hostport string) (authv2.Credential, error) {
			gotHost = hostport
			return authv2.Credential{
				Username:     "user",
				Password:     "pass",
				RefreshToken: "refresh",
				AccessToken:  "access",
			}, nil
		},
	}

	got := authClientFromV2(v2)
	require.NotNil(t, got.CredentialFunc)
	cred, err := got.CredentialFunc(t.Context(), properties.Resource{Registry: "registry.example:5000", Path: "org/chart"})
	require.NoError(t, err)

	assert.Equal(t, "registry.example:5000", gotHost)
	assert.Equal(t, credentials.Credential{
		Username:     "user",
		Password:     "pass",
		RefreshToken: "refresh",
		AccessToken:  "access",
	}, cred)
}

func TestAuthClientFromV2_CredentialError(t *testing.T) {
	wantErr := errors.New("no credential")
	v2 := authv2.Client{
		Credential: func(context.Context, string) (authv2.Credential, error) {
			return authv2.EmptyCredential, wantErr
		},
	}

	_, err := authClientFromV2(v2).CredentialFunc(t.Context(), properties.Resource{Registry: "registry.example"})
	assert.ErrorIs(t, err, wantErr)
}

// newBearerRegistry serves a registry that issues a bearer challenge and a
// token server at /token. It records the method of each token request and
// answers POST with postStatus.
func newBearerRegistry(t *testing.T, postStatus int) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var tokenMethods []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			mu.Lock()
			tokenMethods = append(tokenMethods, r.Method)
			mu.Unlock()
			if r.Method == http.MethodPost && postStatus != http.StatusOK {
				w.WriteHeader(postStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"token","token":"token"}`)
		default:
			if r.Header.Get("Authorization") != "Bearer token" {
				w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="test",scope="repository:org/chart:pull"`, server.URL))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(server.Close)
	return server, &tokenMethods
}

func doBearerRequest(t *testing.T, client *auth.Client, server *httptest.Server) (*http.Response, error) {
	t.Helper()
	// The test server listens on loopback, which the default realm denylist blocks.
	client.BlockedRealmPrefixes = []netip.Prefix{}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/v2/org/chart/tags/list", http.NoBody)
	require.NoError(t, err)
	return client.Do(req)
}

func passwordCredential(context.Context, string) (authv2.Credential, error) {
	return authv2.Credential{Username: "user", Password: "pass"}, nil
}

func TestAuthClientFromV2_TokenFlow(t *testing.T) {
	tests := []struct {
		name               string
		forceAttemptOAuth2 bool
		postStatus         int
		wantMethods        []string
		wantErr            bool
	}{
		{
			name:        "distribution token flow by default",
			postStatus:  http.StatusOK,
			wantMethods: []string{http.MethodGet},
		},
		{
			name:               "OAuth2 when forced",
			forceAttemptOAuth2: true,
			postStatus:         http.StatusOK,
			wantMethods:        []string{http.MethodPost},
		},
		{
			name:               "no distribution fallback when OAuth2 is forced",
			forceAttemptOAuth2: true,
			postStatus:         http.StatusNotFound,
			wantMethods:        []string{http.MethodPost},
			wantErr:            true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, methods := newBearerRegistry(t, tt.postStatus)
			client := authClientFromV2(authv2.Client{
				Credential:         passwordCredential,
				ForceAttemptOAuth2: tt.forceAttemptOAuth2,
			})

			resp, err := doBearerRequest(t, client, server)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}
			assert.Equal(t, tt.wantMethods, *methods)
		})
	}
}

// recordingCache is a v2 cache that records the calls made through it.
type recordingCache struct {
	authv2.Cache
	mu      sync.Mutex
	schemes []authv2.Scheme
}

func (c *recordingCache) Set(ctx context.Context, registry string, scheme authv2.Scheme, key string, fetch func(context.Context) (string, error)) (string, error) {
	c.mu.Lock()
	c.schemes = append(c.schemes, scheme)
	c.mu.Unlock()
	return c.Cache.Set(ctx, registry, scheme, key, fetch)
}

func TestAuthClientFromV2_Cache(t *testing.T) {
	server, methods := newBearerRegistry(t, http.StatusOK)
	cache := &recordingCache{Cache: authv2.NewCache()}
	client := authClientFromV2(authv2.Client{
		Credential: passwordCredential,
		Cache:      cache,
	})

	for range 2 {
		resp, err := doBearerRequest(t, client, server)
		require.NoError(t, err)
		resp.Body.Close()
	}

	assert.Equal(t, []authv2.Scheme{authv2.SchemeBearer}, cache.schemes)
	// The second request reuses the cached token.
	assert.Equal(t, []string{http.MethodGet}, *methods)
}

func TestClientOptAuthorizer_V2(t *testing.T) {
	v2 := authv2.Client{
		Header:     http.Header{"X-Test": {"value"}},
		Credential: passwordCredential,
	}

	client, err := NewClient(ClientOptAuthorizer(v2))
	require.NoError(t, err)

	require.NotNil(t, client.authorizer)
	assert.True(t, client.customHTTPClient)
	assert.Equal(t, "value", client.authorizer.Header.Get("X-Test"))
	cred, err := client.authorizer.CredentialFunc(t.Context(), properties.Resource{Registry: "registry.example"})
	require.NoError(t, err)
	assert.Equal(t, "user", cred.Username)
}
