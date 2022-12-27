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

package getter

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"helm.sh/helm/v3/internal/tlsutil"
	"helm.sh/helm/v3/internal/urlutil"
	"helm.sh/helm/v3/pkg/registry"
)

// OCIGetter is the default HTTP(/S) backend handler
type OCIGetter struct {
	opts      options
	transport *http.Transport
	once      sync.Once
}

// Get performs a Get from repo.Getter and returns the body.
func (g *OCIGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

func (g *OCIGetter) get(href string) (*bytes.Buffer, error) {
	client := g.opts.registryClient
	// if the user has already provided a configured registry client, use it,
	// this is particularly true when user has his own way of handling the client credentials.
	if client == nil {
		c, err := g.newRegistryClient()
		if err != nil {
			return nil, err
		}
		client = c
	}

	ref := strings.TrimPrefix(href, fmt.Sprintf("%s://", registry.OCIScheme))

	var pullOpts []registry.PullOption
	requestingProv := strings.HasSuffix(ref, ".prov")
	if requestingProv {
		ref = strings.TrimSuffix(ref, ".prov")
		pullOpts = append(pullOpts,
			registry.PullOptWithChart(false),
			registry.PullOptWithProv(true))
	}

	result, err := client.Pull(ref, pullOpts...)
	if err != nil {
		return nil, err
	}

	if requestingProv {
		return bytes.NewBuffer(result.Prov.Data), nil
	}
	return bytes.NewBuffer(result.Chart.Data), nil
}

// NewOCIGetter constructs a valid http/https client as a Getter
func NewOCIGetter(ops ...Option) (Getter, error) {
	var client OCIGetter

	for _, opt := range ops {
		opt(&client.opts)
	}

	return &client, nil
}

func (g *OCIGetter) newRegistryClient() (*registry.Client, error) {
	if g.opts.transport != nil {
		client, err := registry.NewClient(
			registry.ClientOptHTTPClient(&http.Client{
				Transport: g.opts.transport,
				Timeout:   g.opts.timeout,
			}),
		)
		if err != nil {
			return nil, err
		}
		return client, nil
	}

	g.once.Do(func() {
		g.transport = &http.Transport{
			// From https://github.com/google/go-containerregistry/blob/31786c6cbb82d6ec4fb8eb79cd9387905130534e/pkg/v1/remote/options.go#L87
			DisableCompression: true,
			DialContext: (&net.Dialer{
				// By default we wrap the transport in retries, so reduce the
				// default dial timeout to 5s to avoid 5x 30s of connection
				// timeouts when doing the "ping" on certain http registries.
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	})

	if (g.opts.certFile != "" && g.opts.keyFile != "") || g.opts.caFile != "" || g.opts.insecureSkipVerifyTLS {
		tlsConf, err := tlsutil.NewClientTLS(g.opts.certFile, g.opts.keyFile, g.opts.caFile, g.opts.insecureSkipVerifyTLS)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %w", err)
		}

		sni, err := urlutil.ExtractHostname(g.opts.url)
		if err != nil {
			return nil, err
		}
		tlsConf.ServerName = sni

		g.transport.TLSClientConfig = tlsConf
	}

	client, err := registry.NewClient(
		registry.ClientOptHTTPClient(&http.Client{
			Transport: g.transport,
			Timeout:   g.opts.timeout,
		}),
	)

	if err != nil {
		return nil, err
	}

	return client, nil
}
