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
	"net/http"
	"strings"

	"helm.sh/helm/v4/internal/version"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type Authorizer struct {
	auth.Client
	AttemptBearerAuthentication bool
}

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

	authorizer.AttemptBearerAuthentication = true
	return &authorizer
}

func (a *Authorizer) EnableCache() {
	a.Cache = auth.NewCache()
}

// Do This method wraps auth.Client.Do in attempt to retry authentication
func (a *Authorizer) Do(originalReq *http.Request) (*http.Response, error) {
	if a.AttemptBearerAuthentication {
		needsAuthentication := originalReq.Header.Get("Authorization") == ""
		if needsAuthentication {
			a.ForceAttemptOAuth2 = true
			if originalReq.Host == "ghcr.io" {
				a.ForceAttemptOAuth2 = false
				a.AttemptBearerAuthentication = false
			}
			resp, err := a.Client.Do(originalReq)
			if err == nil {
				a.AttemptBearerAuthentication = false
				return resp, nil
			}
			if !strings.Contains(err.Error(), "response status code 40") {
				return nil, err
			}
		}
	}
	return a.Client.Do(originalReq)
}
