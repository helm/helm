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

	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
	authv2 "oras.land/oras-go/v2/registry/remote/auth"
)

// authClientFromV2 converts an oras-go v2 auth client, as accepted by the
// deprecated ClientOptAuthorizer, into the oras-go v3 client the registry
// client uses.
//
// Client, Header and ClientID carry over unchanged. The v2 credential function
// is called with the registry host of the v3 resource, which is the host:port
// the v2 client passed it. A v2 cache is used through cacheFromV2, so a cache
// shared between clients stays shared. ForceAttemptOAuth2 selects the token
// flow: false keeps v2's distribution token flow, and true uses OAuth2 without
// falling back to it, as v2 did.
func authClientFromV2(c authv2.Client) *auth.Client {
	client := &auth.Client{
		Client:   c.Client,
		Header:   c.Header.Clone(),
		ClientID: c.ClientID,
	}
	if c.Credential != nil {
		credential := c.Credential
		client.CredentialFunc = func(ctx context.Context, resource properties.Resource) (credentials.Credential, error) {
			cred, err := credential(ctx, resource.Registry)
			return credentials.Credential(cred), err
		}
	}
	if c.Cache != nil {
		client.Cache = cacheFromV2{c.Cache}
	}
	fetcher := auth.NewCompositeTokenFetcher(client.Client, client.Header, client.ClientID, !c.ForceAttemptOAuth2)
	fetcher.StrictOAuth2 = c.ForceAttemptOAuth2
	client.TokenFetcher = fetcher
	return client
}

// cacheFromV2 adapts an oras-go v2 auth cache to the v3 interface, which has
// the same methods and the same Scheme values.
type cacheFromV2 struct {
	cache authv2.Cache
}

func (c cacheFromV2) GetScheme(ctx context.Context, registry string) (auth.Scheme, error) {
	scheme, err := c.cache.GetScheme(ctx, registry)
	return auth.Scheme(scheme), err
}

func (c cacheFromV2) GetToken(ctx context.Context, registry string, scheme auth.Scheme, key string) (string, error) {
	return c.cache.GetToken(ctx, registry, authv2.Scheme(scheme), key)
}

func (c cacheFromV2) Set(ctx context.Context, registry string, scheme auth.Scheme, key string, fetch func(context.Context) (string, error)) (string, error) {
	return c.cache.Set(ctx, registry, authv2.Scheme(scheme), key, fetch)
}
