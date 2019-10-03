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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"io"
)

type (
	// ClientOption allows specifying various settings configurable by the user for overriding the defaults
	// used when creating a new default client
	ClientOption func(*Client)
)

// ClientOptDebug returns a function that sets the debug setting on client options set
func ClientOptDebug(debug bool) ClientOption {
	return func(client *Client) {
		client.debug = debug
	}
}

// ClientOptWriter returns a function that sets the writer setting on client options set
func ClientOptWriter(out io.Writer) ClientOption {
	return func(client *Client) {
		client.out = out
	}
}

// ClientOptResolver returns a function that sets the resolver setting on client options set
func ClientOptResolver(resolver *Resolver) ClientOption {
	return func(client *Client) {
		client.resolver = resolver
	}
}

// ClientOptAuthorizer returns a function that sets the authorizer setting on client options set
func ClientOptAuthorizer(authorizer *Authorizer) ClientOption {
	return func(client *Client) {
		client.authorizer = authorizer
	}
}

// ClientOptCache returns a function that sets the cache setting on a client options set
func ClientOptCache(cache *Cache) ClientOption {
	return func(client *Client) {
		client.cache = cache
	}
}
