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
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/internal/urlutil"
	"helm.sh/helm/v4/pkg/registry"
)

// OCIGetter is the default HTTP(/S) backend handler
type OCIGetter struct {
	opts      getterOptions
	transport *http.Transport
	once      sync.Once
}

// Get performs a Get from repo.Getter and returns the body.
func (g *OCIGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	// Create a local copy of options to avoid data races when Get is called concurrently
	opts := g.opts
	for _, opt := range options {
		opt(&opts)
	}
	return g.get(href, opts)
}

func (g *OCIGetter) get(href string, opts getterOptions) (*bytes.Buffer, error) {
	client := opts.registryClient
	// if the user has already provided a configured registry client, use it,
	// this is particularly true when user has his own way of handling the client credentials.
	if client == nil {
		c, err := g.newRegistryClient(opts)
		if err != nil {
			return nil, err
		}
		client = c
	}

	ref := strings.TrimPrefix(href, registry.OCIScheme+"://")

	if version := opts.version; version != "" && !strings.Contains(path.Base(ref), ":") {
		ref = fmt.Sprintf("%s:%s", ref, version)
	}
	// Check if this is a plugin request
	if opts.artifactType == "plugin" {
		return g.getPlugin(client, ref)
	}

	// Default to chart behavior for backward compatibility
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

func newOCITransport(tlsConf *tls.Config) *http.Transport {
	return &http.Transport{
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
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       tlsConf,
	}
}

func (g *OCIGetter) newRegistryClient(opts getterOptions) (*registry.Client, error) {
	if opts.transport != nil {
		client, err := registry.NewClient(
			registry.ClientOptHTTPClient(&http.Client{
				Transport: opts.transport,
				Timeout:   opts.timeout,
			}),
		)
		if err != nil {
			return nil, err
		}
		return client, nil
	}

	var transport *http.Transport
	needsCustomTLS := (opts.certFile != "" && opts.keyFile != "") || opts.caFile != "" || opts.insecureSkipVerifyTLS
	if needsCustomTLS {
		// Create a new transport for custom TLS to avoid mutating the shared transport
		// and causing data races or leaking TLS settings across calls.
		tlsConf, err := tlsutil.NewTLSConfig(
			tlsutil.WithInsecureSkipVerify(opts.insecureSkipVerifyTLS),
			tlsutil.WithCertKeyPairFiles(opts.certFile, opts.keyFile),
			tlsutil.WithCAFile(opts.caFile),
		)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %w", err)
		}

		sni, err := urlutil.ExtractHostname(opts.url)
		if err != nil {
			return nil, err
		}
		tlsConf.ServerName = sni

		transport = newOCITransport(tlsConf)
	} else {
		// Use shared transport for default case (no custom TLS)
		g.once.Do(func() {
			g.transport = newOCITransport(&tls.Config{})
		})
		transport = g.transport
	}

	clientOpts := []registry.ClientOption{registry.ClientOptHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   opts.timeout,
	})}
	if opts.plainHTTP {
		clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
	}

	client, err := registry.NewClient(clientOpts...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// getPlugin handles plugin-specific OCI pulls
func (g *OCIGetter) getPlugin(client *registry.Client, ref string) (*bytes.Buffer, error) {
	// Check if this is a provenance file request
	requestingProv := strings.HasSuffix(ref, ".prov")
	if requestingProv {
		ref = strings.TrimSuffix(ref, ".prov")
	}

	// Extract plugin name from the reference
	// e.g., "ghcr.io/user/plugin-name:v1.0.0" -> "plugin-name"
	parts := strings.Split(ref, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid OCI reference: %s", ref)
	}
	lastPart := parts[len(parts)-1]
	pluginName := lastPart
	if idx := strings.LastIndex(lastPart, ":"); idx > 0 {
		pluginName = lastPart[:idx]
	}
	if idx := strings.LastIndex(lastPart, "@"); idx > 0 {
		pluginName = lastPart[:idx]
	}

	var pullOpts []registry.PluginPullOption
	if requestingProv {
		pullOpts = append(pullOpts, registry.PullPluginOptWithProv(true))
	}

	result, err := client.PullPlugin(ref, pluginName, pullOpts...)
	if err != nil {
		return nil, err
	}

	if requestingProv {
		return bytes.NewBuffer(result.Prov.Data), nil
	}
	return bytes.NewBuffer(result.PluginData), nil
}
