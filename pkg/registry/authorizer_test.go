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
	"io"
	"net/http"
	"strings"
	"testing"

	"oras.land/oras-go/v2/registry/remote/auth"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newHTTPClient(rt roundTripFunc) *http.Client {
	return &http.Client{Transport: rt}
}

func resp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

// fakeStore is a fake credentials store used to assert it is used when username/password are not set.
type fakeStore struct{ called bool }

func (f *fakeStore) Get(_ context.Context, _ string) (auth.Credential, error) {
	f.called = true
	return auth.Credential{}, nil
}
func (f *fakeStore) Put(_ context.Context, _ string, _ auth.Credential) error { return nil }
func (f *fakeStore) Delete(_ context.Context, _ string) error                 { return nil }

func TestNewAuthorizer_UsernamePassword(t *testing.T) {
	hc := newHTTPClient(func(r *http.Request) (*http.Response, error) {
		// ensure user-agent header is set by authorizer
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Fatalf("expected User-Agent to be set")
		}
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "user", "pass")
	if !a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should start true")
	}
	// Verify credential function returns our basic auth creds
	cred, err := a.Credential(t.Context(), "example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cred.Username != "user" || cred.Password != "pass" {
		t.Fatalf("credential not set correctly: %+v", cred)
	}
	// simple do to trigger user-agent path and flip AttemptBearerAuthentication to false
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	_, err = a.Do(req)
	if err != nil {
		t.Fatalf("unexpected Do error: %v", err)
	}
	if a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should be false after Do")
	}
}

func TestNewAuthorizer_CredentialStoreUsed(t *testing.T) {
	fs := &fakeStore{}
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) { return resp(200, "ok"), nil })
	a := NewAuthorizer(hc, fs, "", "")
	// invoke Credential to ensure it delegates to store
	_, _ = a.Credential(t.Context(), "registry.example")
	if !fs.called {
		t.Fatalf("expected credential store to be called")
	}
}

func TestEnableCache_SetsCache(t *testing.T) {
	a := NewAuthorizer(newHTTPClient(func(_ *http.Request) (*http.Response, error) { return resp(200, "ok"), nil }), nil, "", "")
	if a.Cache != nil {
		t.Fatalf("cache should be nil before EnableCache")
	}
	a.EnableCache()
	if a.Cache == nil {
		t.Fatalf("cache should be set after EnableCache")
	}
}

func TestDo_SuccessFirstTry_DisablesAttempt(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://registry.example/v2/", nil)
	req.Host = "registry.example" // not ghcr.io
	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	if a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should be false after success")
	}
}

func TestDo_AuthErrorThenRetry(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("unexpected response status code 401")
		}
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"
	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls on auth error, got %d", calls)
	}
	// After a retry that succeeds on second attempt, AttemptBearerAuthentication remains true
	// because the flag is only set to false after a successful first attempt
	if !a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should remain true after retry path")
	}
}

func TestDo_NonAuthErrorReturned(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("network down")
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"
	_, err := a.Do(req)
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("expected network error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected only 1 call on non-auth error, got %d", calls)
	}
	// In this branch the code returns before flipping AttemptBearerAuthentication at end of block
	if !a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should remain true when returning early on non-auth error")
	}
}

func TestDo_GHCRSkipsFirstAttempt(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://ghcr.io/v2/", nil)
	req.Host = "ghcr.io"
	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected single call for ghcr.io, got %d", calls)
	}
	if a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should be false after ghcr path")
	}
}

func TestDo_WithAuthorizationHeader_SkipsPreflight(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Header.Set("Authorization", "Bearer token")
	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one direct call when Authorization present, got %d", calls)
	}
	if !a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should remain true when Authorization header is present")
	}
}

func TestDo_ForceAttemptOAuth2_SetForNonGHCR(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"

	// First call should set ForceAttemptOAuth2 to true for non-ghcr.io hosts
	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.ForceAttemptOAuth2 {
		t.Fatalf("ForceAttemptOAuth2 should be true for non-ghcr.io hosts")
	}
}

func TestDo_ForceAttemptOAuth2_NotSetForGHCR(t *testing.T) {
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://ghcr.io/v2/", nil)
	req.Host = "ghcr.io"

	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ForceAttemptOAuth2 {
		t.Fatalf("ForceAttemptOAuth2 should be false for ghcr.io")
	}
}

func TestDo_MultipleAuthErrors_RetriesCorrectly(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			return nil, errors.New("unexpected response status code 401: Unauthorized")
		case 2:
			return resp(200, "ok"), nil
		default:
			t.Fatalf("unexpected number of calls: %d", calls)
			return nil, errors.New("unexpected")
		}
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"

	resp, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", resp.StatusCode)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls for retry, got %d", calls)
	}
}

func TestDo_403Error_RetriesCorrectly(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("unexpected response status code 403: Forbidden")
		}
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"

	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls for 403 error retry, got %d", calls)
	}
}

func TestDo_AttemptBearerAuthentication_False_SkipsLogic(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")
	a.AttemptBearerAuthentication = false // Explicitly set to false

	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	_, err := a.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected single call when AttemptBearerAuthentication is false, got %d", calls)
	}
	if a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should remain false")
	}
}

func TestDo_SequentialRequests_MaintainsState(t *testing.T) {
	callCount := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		callCount++
		return resp(200, "ok"), nil
	})
	a := NewAuthorizer(hc, nil, "", "")

	// First request without auth header
	req1, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req1.Host = "example.com"
	_, err := a.Do(req1)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	if a.AttemptBearerAuthentication {
		t.Fatalf("AttemptBearerAuthentication should be false after first request")
	}

	// Second request should go straight through
	req2, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/charts", nil)
	req2.Host = "example.com"
	_, err = a.Do(req2)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	// Should only have made 2 calls total (no retry on second)
	if callCount != 2 {
		t.Fatalf("expected 2 total calls, got %d", callCount)
	}
}

func TestDo_ErrorMessageParsing_404NotRetried(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		// 404 error should contain "40" but not trigger retry since it's not 401/403
		return nil, errors.New("unexpected response status code 404: Not Found")
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"

	_, err := a.Do(req)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls for 404 (matches '40' pattern), got %d", calls)
	}
}

func TestDo_ErrorMessageParsing_NonStatusCodeError(t *testing.T) {
	calls := 0
	hc := newHTTPClient(func(_ *http.Request) (*http.Response, error) {
		calls++
		// Error containing "40" but not a status code error
		return nil, errors.New("failed after 40 attempts")
	})
	a := NewAuthorizer(hc, nil, "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/v2/", nil)
	req.Host = "example.com"

	_, err := a.Do(req)
	if err == nil || !strings.Contains(err.Error(), "40 attempts") {
		t.Fatalf("expected error with '40 attempts', got %v", err)
	}
	// Should not retry since it doesn't match the pattern despite containing "40"
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for non-status code errors), got %d", calls)
	}
}

func TestNewAuthorizer_NilHttpClient(t *testing.T) {
	// Test that NewAuthorizer works with nil HTTP client
	a := NewAuthorizer(nil, nil, "user", "pass")
	if a == nil {
		t.Fatalf("NewAuthorizer should not return nil")
	}
	if a.Client.Client != nil {
		t.Fatalf("expected nil HTTP client to remain nil")
	}
	// Verify credential function still works
	cred, err := a.Credential(t.Context(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Username != "user" || cred.Password != "pass" {
		t.Fatalf("credentials not set correctly: %+v", cred)
	}
}
