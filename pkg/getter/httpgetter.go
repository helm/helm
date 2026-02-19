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
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/internal/version"
)

// HTTPGetter is the default HTTP(/S) backend handler
type HTTPGetter struct {
	opts      getterOptions
	transport *http.Transport
	once      sync.Once
}

// Get performs a Get from repo.Getter and returns the body.
func (g *HTTPGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	// Create a local copy of options to avoid data races when Get is called concurrently
	opts := g.opts
	for _, opt := range options {
		opt(&opts)
	}
	return g.get(href, opts)
}

func (g *HTTPGetter) get(href string, opts getterOptions) (*bytes.Buffer, error) {
	// Set a helm specific user agent so that a repo server and metrics can
	// separate helm calls from other tools interacting with repos.
	req, err := http.NewRequest(http.MethodGet, href, nil)
	if err != nil {
		return nil, err
	}

	if opts.acceptHeader != "" {
		req.Header.Set("Accept", opts.acceptHeader)
	}

	req.Header.Set("User-Agent", version.GetUserAgent())
	if opts.userAgent != "" {
		req.Header.Set("User-Agent", opts.userAgent)
	}

	// Before setting the basic auth credentials, make sure the URL associated
	// with the basic auth is the one being fetched.
	u1, err := url.Parse(opts.url)
	if err != nil {
		return nil, fmt.Errorf("unable to parse getter URL: %w", err)
	}
	u2, err := url.Parse(href)
	if err != nil {
		return nil, fmt.Errorf("unable to parse URL getting from: %w", err)
	}

	// Host on URL (returned from url.Parse) contains the port if present.
	// This check ensures credentials are not passed between different
	// services on different ports.
	if opts.passCredentialsAll || (u1.Scheme == u2.Scheme && u1.Host == u2.Host) {
		if opts.username != "" && opts.password != "" {
			req.SetBasicAuth(opts.username, opts.password)
		}
	}

	client, err := g.httpClient(opts)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s : %s", href, resp.Status)
	}

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	return buf, err
}

// NewHTTPGetter constructs a valid http/https client as a Getter
func NewHTTPGetter(options ...Option) (Getter, error) {
	var client HTTPGetter

	for _, opt := range options {
		opt(&client.opts)
	}

	return &client, nil
}

func (g *HTTPGetter) httpClient(opts getterOptions) (*http.Client, error) {
	if opts.transport != nil {
		return &http.Client{
			Transport: opts.transport,
			Timeout:   opts.timeout,
		}, nil
	}

	// Check if we need custom TLS configuration
	needsCustomTLS := (opts.certFile != "" && opts.keyFile != "") || opts.caFile != "" || opts.insecureSkipVerifyTLS

	if needsCustomTLS {
		// Create a new transport for custom TLS to avoid race conditions
		transport := &http.Transport{
			DisableCompression: true,
			Proxy:              http.ProxyFromEnvironment,
		}

		tlsConf, err := tlsutil.NewTLSConfig(
			tlsutil.WithInsecureSkipVerify(opts.insecureSkipVerifyTLS),
			tlsutil.WithCertKeyPairFiles(opts.certFile, opts.keyFile),
			tlsutil.WithCAFile(opts.caFile),
		)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %w", err)
		}

		transport.TLSClientConfig = tlsConf

		return &http.Client{
			Transport: transport,
			Timeout:   opts.timeout,
		}, nil
	}

	// Use shared transport for default case (no custom TLS)
	g.once.Do(func() {
		g.transport = &http.Transport{
			DisableCompression: true,
			Proxy:              http.ProxyFromEnvironment,
			TLSClientConfig:    &tls.Config{},
		}
	})

	return &http.Client{
		Transport: g.transport,
		Timeout:   opts.timeout,
	}, nil
}
