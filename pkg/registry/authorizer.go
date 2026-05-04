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
	"net"
	"net/http"
	"strings"
	"sync"

	"helm.sh/helm/v4/internal/version"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// Authorizer wraps auth.Client to retry authentication with bearer-to-basic fallback on 401/403.
type Authorizer struct {
	auth.Client
	lock                        sync.RWMutex
	attemptBearerAuthentication bool
}

// isGHCR reports whether host refers to ghcr.io, normalizing port and case.
func isGHCR(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	return strings.EqualFold(h, "ghcr.io")
}

// reqHost returns the effective host for an outbound request, preferring URL.Hostname() over req.Host.
func reqHost(req *http.Request) string {
	if h := req.URL.Hostname(); h != "" {
		return h
	}
	return req.Host
}

// NewAuthorizer creates an Authorizer backed by the given HTTP client and credentials store.
func NewAuthorizer(httpClient *http.Client, credentialsStore credentials.Store, username, password string) *Authorizer {
	authorizer := Authorizer{
		Client: auth.Client{
			Client: httpClient,
		},
	}
	authorizer.SetUserAgent(version.GetUserAgent())

	if username != "" && password != "" {
		authorizer.Credential = func(_ context.Context, _ string) (auth.Credential, error) {
			return auth.Credential{Username: username, Password: password}, nil
		}
	} else {
		authorizer.Credential = credentials.Credential(credentialsStore)
	}

	authorizer.setAttemptBearerAuthentication(true)
	return &authorizer
}

// EnableCache enables per-host token caching on the underlying auth client.
func (a *Authorizer) EnableCache() {
	a.Cache = auth.NewCache()
}

func (a *Authorizer) getAttemptBearerAuthentication() bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.attemptBearerAuthentication
}

func (a *Authorizer) setAttemptBearerAuthentication(value bool) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.attemptBearerAuthentication = value
}

func (a *Authorizer) getForceAttemptOAuth2() bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.ForceAttemptOAuth2
}

func (a *Authorizer) setForceAttemptOAuth2(value bool) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.ForceAttemptOAuth2 = value
}

// Do wraps auth.Client.Do to retry with OAuth2 bearer forced on 401/403 errors.
// The first attempt uses standard auth; only on 401/403 failure do we retry with
// ForceAttemptOAuth2=true to fix registries (e.g. Quay) whose token endpoints
// require OAuth2-style requests.
func (a *Authorizer) Do(originalReq *http.Request) (*http.Response, error) {
	needsAuthentication := originalReq.Header.Get("Authorization") == ""

	if needsAuthentication && a.getAttemptBearerAuthentication() && isGHCR(reqHost(originalReq)) {
		a.setForceAttemptOAuth2(false)
		a.setAttemptBearerAuthentication(false)
	}

	resp, err := a.Client.Do(originalReq)
	if err == nil {
		if needsAuthentication {
			a.setAttemptBearerAuthentication(false)
		}
		return resp, nil
	}

	if !needsAuthentication || !a.getAttemptBearerAuthentication() {
		return nil, err
	}

	if !strings.Contains(err.Error(), "response status code 401") &&
		!strings.Contains(err.Error(), "response status code 403") {
		return nil, err
	}

	// Standard auth failed with 401/403; retry forcing OAuth2 bearer flow.
	prev := a.getForceAttemptOAuth2()
	a.setForceAttemptOAuth2(true)
	defer a.setForceAttemptOAuth2(prev)

	resp, err = a.Client.Do(originalReq)
	if err == nil {
		a.setAttemptBearerAuthentication(false)
	}
	return resp, err
}
