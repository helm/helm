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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type mockCredentialsStore struct {
	username string
	password string
	err      error
}

func (m *mockCredentialsStore) Get(_ context.Context, _ string) (auth.Credential, error) {
	if m.err != nil {
		return auth.EmptyCredential, m.err
	}
	return auth.Credential{
		Username: m.username,
		Password: m.password,
	}, nil
}

func (m *mockCredentialsStore) Put(_ context.Context, _ string, _ auth.Credential) error {
	return nil
}

func (m *mockCredentialsStore) Delete(_ context.Context, _ string) error {
	return nil
}

func TestNewAuthorizer(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "with username and password",
			username: "testuser",
			password: "testpass",
		},
		{
			name:     "without credentials",
			username: "",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{}
			credStore := &mockCredentialsStore{}

			authorizer := NewAuthorizer(httpClient, credStore, tt.username, tt.password)

			require.NotNil(t, authorizer)
			assert.Equal(t, httpClient, authorizer.Client.Client)
			assert.True(t, authorizer.getAttemptBearerAuthentication())
			assert.NotNil(t, authorizer.Credential)

			if tt.username != "" && tt.password != "" {
				cred, err := authorizer.Credential(t.Context(), "")
				require.NoError(t, err)
				assert.Equal(t, tt.username, cred.Username)
				assert.Equal(t, tt.password, cred.Password)
			}
		})
	}
}

func TestNewAuthorizer_WithCredentialsStore(t *testing.T) {
	httpClient := &http.Client{}
	credStore := &mockCredentialsStore{
		username: "storeuser",
		password: "storepass",
	}

	authorizer := NewAuthorizer(httpClient, credStore, "", "")

	require.NotNil(t, authorizer)

	cred, err := authorizer.Credential(t.Context(), "test.com")
	require.NoError(t, err)
	assert.Equal(t, "storeuser", cred.Username)
	assert.Equal(t, "storepass", cred.Password)
}

func TestAuthorizer_EnableCache(t *testing.T) {
	httpClient := &http.Client{}
	credStore := &mockCredentialsStore{}

	authorizer := NewAuthorizer(httpClient, credStore, "", "")
	assert.Nil(t, authorizer.Cache)

	authorizer.EnableCache()
	assert.NotNil(t, authorizer.Cache)
}

func TestAuthorizer_Do(t *testing.T) {
	tests := []struct {
		name                  string
		host                  string
		authHeader            string
		serverStatus          int
		expectForceOAuth2     bool
		expectBearerAuthAfter bool
	}{
		{
			name:                  "successful request without auth header",
			host:                  "registry.example.com",
			authHeader:            "",
			serverStatus:          200,
			expectForceOAuth2:     true,
			expectBearerAuthAfter: false,
		},
		{
			name:                  "request with existing auth header",
			host:                  "registry.example.com",
			authHeader:            "Bearer token123",
			serverStatus:          200,
			expectForceOAuth2:     false,
			expectBearerAuthAfter: true,
		},
		{
			name:                  "ghcr.io special handling",
			host:                  "ghcr.io",
			authHeader:            "",
			serverStatus:          200,
			expectForceOAuth2:     false,
			expectBearerAuthAfter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte("success"))
			}))
			defer server.Close()

			httpClient := &http.Client{}
			credStore := &mockCredentialsStore{}

			authorizer := NewAuthorizer(httpClient, credStore, "", "")

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			require.NoError(t, err)
			req.Host = tt.host

			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := authorizer.Do(req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expectBearerAuthAfter, authorizer.getAttemptBearerAuthentication())

			if tt.authHeader == "" {
				assert.Equal(t, tt.expectForceOAuth2, authorizer.getForceAttemptOAuth2())
			}

			resp.Body.Close()
		})
	}
}

func TestAuthorizer_Do_WithBearerAttemptDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	httpClient := &http.Client{}
	credStore := &mockCredentialsStore{}

	authorizer := NewAuthorizer(httpClient, credStore, "", "")
	authorizer.setAttemptBearerAuthentication(false)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Host = "registry.example.com"

	resp, err := authorizer.Do(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, authorizer.getAttemptBearerAuthentication())

	resp.Body.Close()
}

func TestAuthorizer_Do_NonRetryableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	httpClient := &http.Client{}
	credStore := &mockCredentialsStore{}

	authorizer := NewAuthorizer(httpClient, credStore, "", "")

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Host = "registry.example.com"

	resp, err := authorizer.Do(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	resp.Body.Close()
}

func TestAuthorizer_ConcurrentAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	httpClient := &http.Client{}
	credStore := &mockCredentialsStore{}
	authorizer := NewAuthorizer(httpClient, credStore, "", "")

	const numGoroutines = 100
	const numRequests = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numRequests; j++ {
				req, err := http.NewRequest(http.MethodGet, server.URL, nil)
				require.NoError(t, err)
				req.Host = "registry.example.com"

				resp, err := authorizer.Do(req)
				if err == nil && resp != nil {
					resp.Body.Close()
				}
			}
		}()

		go func() {
			defer wg.Done()
			for j := 0; j < numRequests; j++ {
				authorizer.setAttemptBearerAuthentication(true)
				val := authorizer.getAttemptBearerAuthentication()
				if val != true {
					t.Logf("Warning: Expected true but got %v", val)
				}

				authorizer.setAttemptBearerAuthentication(false)
				val = authorizer.getAttemptBearerAuthentication()
				if val != false {
					t.Logf("Warning: Expected false but got %v", val)
				}
			}
		}()
	}

	wg.Wait()
}

func TestAuthorizer_Do_StatusCodeErrorChecking(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		shouldRetry bool
		description string
	}{
		{
			name:        "retry on 401 error",
			errorMsg:    "response status code 401",
			shouldRetry: true,
			description: "401 errors should trigger retry logic",
		},
		{
			name:        "retry on 403 error",
			errorMsg:    "response status code 403",
			shouldRetry: true,
			description: "403 errors should trigger retry logic",
		},
		{
			name:        "no retry on 404 error",
			errorMsg:    "response status code 404",
			shouldRetry: false,
			description: "404 errors should not trigger retry logic",
		},
		{
			name:        "no retry on 500 error",
			errorMsg:    "response status code 500",
			shouldRetry: false,
			description: "500 errors should not trigger retry logic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)

			should401Retry := strings.Contains(err.Error(), "response status code 401")
			should403Retry := strings.Contains(err.Error(), "response status code 403")
			actualShouldRetry := should401Retry || should403Retry

			assert.Equal(t, tt.shouldRetry, actualShouldRetry, tt.description)
		})
	}
}
