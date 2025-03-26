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
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/internal/urlutil"
	"helm.sh/helm/v4/internal/version"
)

// HTTPGetter is the default HTTP(/S) backend handler
type HTTPGetter struct {
	opts      options
	transport *http.Transport
	once      sync.Once
}

// Get performs a Get from repo.Getter and returns the body.
func (g *HTTPGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

func (g *HTTPGetter) get(href string) (*bytes.Buffer, error) {
	// Set a helm specific user agent so that a repo server and metrics can
	// separate helm calls from other tools interacting with repos.
	req, err := http.NewRequest(http.MethodGet, href, nil)
	if err != nil {
		return nil, err
	}

	if g.opts.acceptHeader != "" {
		req.Header.Set("Accept", g.opts.acceptHeader)
	}

	req.Header.Set("User-Agent", version.GetUserAgent())
	if g.opts.userAgent != "" {
		req.Header.Set("User-Agent", g.opts.userAgent)
	}

	// Before setting the basic auth credentials, make sure the URL associated
	// with the basic auth is the one being fetched.
	u1, err := url.Parse(g.opts.url)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to parse getter URL")
	}
	u2, err := url.Parse(href)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to parse URL getting from")
	}

	// Host on URL (returned from url.Parse) contains the port if present.
	// This check ensures credentials are not passed between different
	// services on different ports.
	if g.opts.passCredentialsAll || (u1.Scheme == u2.Scheme && u1.Host == u2.Host) {
		if g.opts.username != "" && g.opts.password != "" {
			req.SetBasicAuth(g.opts.username, g.opts.password)
		}
	}

	client, err := g.httpClient()
	if err != nil {
		return nil, err
	}

	// Define the maximum number of retries
	maxRetries := 20

	// Define retryable status codes
	retryableCodes := map[int]bool{
		http.StatusBadGateway:         true, // 502
		http.StatusServiceUnavailable: true, // 503
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := client.Do(req)

		if err != nil {
			if attempt < (maxRetries - 1) {
				fmt.Printf("Error: %v\n", err)
				// Sleep for a short duration before retrying
				time.Sleep(1 * time.Second)
				continue
			}

			return nil, err
		}

		respStatus := resp.Status
		defer resp.Body.Close()

		// Check the status code
		if _, ok := retryableCodes[resp.StatusCode]; ok {
			if attempt < (maxRetries - 1) {
				fmt.Printf("Received retryable status code: %v\n", resp.StatusCode)
				// Close the response body
				resp.Body.Close()
				// Sleep for a short duration before retrying
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, errors.Errorf("failed to fetch %s : %s", href, respStatus)
		}

		if resp.StatusCode != http.StatusOK {
			if attempt < (maxRetries - 1) {
				fmt.Printf("Received non-OK status code: %v\n", resp.StatusCode)
				// Close the response body
				resp.Body.Close()
				// Sleep for a short duration before retrying
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, errors.Errorf("failed to fetch %s : %s", href, respStatus)
		}

		// Process the response
		buf := bytes.NewBuffer(nil)
		_, err = io.Copy(buf, resp.Body)
		return buf, err
	}

	return nil, fmt.Errorf("maximum retries exceeded")
}

// NewHTTPGetter constructs a valid http/https client as a Getter
func NewHTTPGetter(options ...Option) (Getter, error) {
	var client HTTPGetter

	for _, opt := range options {
		opt(&client.opts)
	}

	return &client, nil
}

func (g *HTTPGetter) httpClient() (*http.Client, error) {
	if g.opts.transport != nil {
		return &http.Client{
			Transport: g.opts.transport,
			Timeout:   g.opts.timeout,
		}, nil
	}

	g.once.Do(func() {
		g.transport = &http.Transport{
			DisableCompression: true,
			Proxy:              http.ProxyFromEnvironment,
		}
	})

	if (g.opts.certFile != "" && g.opts.keyFile != "") || g.opts.caFile != "" || g.opts.insecureSkipVerifyTLS {
		tlsConf, err := tlsutil.NewTLSConfig(
			tlsutil.WithInsecureSkipVerify(g.opts.insecureSkipVerifyTLS),
			tlsutil.WithCertKeyPairFiles(g.opts.certFile, g.opts.keyFile),
			tlsutil.WithCAFile(g.opts.caFile),
		)
		if err != nil {
			return nil, errors.Wrap(err, "can't create TLS config for client")
		}

		sni, err := urlutil.ExtractHostname(g.opts.url)
		if err != nil {
			return nil, err
		}
		tlsConf.ServerName = sni

		g.transport.TLSClientConfig = tlsConf
	}

	if g.opts.insecureSkipVerifyTLS {
		if g.transport.TLSClientConfig == nil {
			g.transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			g.transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	client := &http.Client{
		Transport: g.transport,
		Timeout:   g.opts.timeout,
	}

	return client, nil
}
