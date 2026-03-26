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

	"github.com/google/uuid"

	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/internal/version"
)

// 🔥 Constant for session header
const helmSessionHeader = "helm-session"

// HTTPGetter is the default HTTP(/S) backend handler
type HTTPGetter struct {
	opts      getterOptions
	transport *http.Transport
	once      sync.Once
	sessionID string // 🔥 Stores session ID for request grouping
}

// Get performs a Get from repo.Getter and returns the body.
func (g *HTTPGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	opts := g.opts
	for _, opt := range options {
		opt(&opts)
	}
	return g.get(href, opts)
}

func (g *HTTPGetter) get(href string, opts getterOptions) (*bytes.Buffer, error) {
	req, err := http.NewRequest(http.MethodGet, href, nil)
	if err != nil {
		return nil, err
	}

	// 🔥 Add Helm session header to group requests from a single command execution
	if g.sessionID != "" {
		req.Header.Set(helmSessionHeader, g.sessionID)
	}

	if opts.acceptHeader != "" {
		req.Header.Set("Accept", opts.acceptHeader)
	}

	req.Header.Set("User-Agent", version.GetUserAgent())
	if opts.userAgent != "" {
		req.Header.Set("User-Agent", opts.userAgent)
	}

	u1, err := url.Parse(opts.url)
	if err != nil {
		return nil, fmt.Errorf("unable to parse getter URL: %w", err)
	}
	u2, err := url.Parse(href)
	if err != nil {
		return nil, fmt.Errorf("unable to parse URL getting from: %w", err)
	}

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

	// 🔥 Generate session ID once per getter instance
	client.sessionID = uuid.New().String()

	return &client, nil
}

func (g *HTTPGetter) httpClient(opts getterOptions) (*http.Client, error) {
	if opts.transport != nil {
		return &http.Client{
			Transport: opts.transport,
			Timeout:   opts.timeout,
		}, nil
	}

	needsCustomTLS := (opts.certFile != "" && opts.keyFile != "") || opts.caFile != "" || opts.insecureSkipVerifyTLS

	if needsCustomTLS {
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
