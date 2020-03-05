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
	"io"
	"net/http"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/tlsutil"
	"helm.sh/helm/v3/internal/urlutil"
	"helm.sh/helm/v3/internal/version"
)

// HTTPGetter is the default HTTP(/S) backend handler
type HTTPGetter struct {
	opts options
}

//Get performs a Get from repo.Getter and returns the body.
func (g *HTTPGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

func (g *HTTPGetter) get(href string) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)

	// Set a helm specific user agent so that a repo server and metrics can
	// separate helm calls from other tools interacting with repos.
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return buf, err
	}

	req.Header.Set("User-Agent", version.GetUserAgent())
	if g.opts.userAgent != "" {
		req.Header.Set("User-Agent", g.opts.userAgent)
	}

	if g.opts.username != "" && g.opts.password != "" {
		req.SetBasicAuth(g.opts.username, g.opts.password)
	}

	client, err := g.httpClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return buf, err
	}
	if resp.StatusCode != 200 {
		return buf, errors.Errorf("failed to fetch %s : %s", href, resp.Status)
	}

	_, err = io.Copy(buf, resp.Body)
	resp.Body.Close()
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

func (g *HTTPGetter) httpClient() (*http.Client, error) {
	if (g.opts.certFile != "" && g.opts.keyFile != "") || g.opts.caFile != "" {
		tlsConf, err := tlsutil.NewClientTLS(g.opts.certFile, g.opts.keyFile, g.opts.caFile)
		if err != nil {
			return nil, errors.Wrap(err, "can't create TLS config for client")
		}
		tlsConf.BuildNameToCertificate()

		sni, err := urlutil.ExtractHostname(g.opts.url)
		if err != nil {
			return nil, err
		}
		tlsConf.ServerName = sni

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
				Proxy:           http.ProxyFromEnvironment,
			},
		}

		return client, nil
	}

	if g.opts.insecureSkipVerifyTLS {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: http.ProxyFromEnvironment,
			},
		}

		return client, nil
	}

	return http.DefaultClient, nil
}
