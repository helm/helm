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

package pusher

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/tlsutil"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/time/ctime"
)

// OCIPusher is the default OCI backend handler
type OCIPusher struct {
	opts options
}

// Push performs a Push from repo.Pusher.
func (pusher *OCIPusher) Push(chartRef, href string, options ...Option) error {
	for _, opt := range options {
		opt(&pusher.opts)
	}
	return pusher.push(chartRef, href)
}

func (pusher *OCIPusher) push(chartRef, href string) error {
	stat, err := os.Stat(chartRef)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("%s: no such file", chartRef)
		}
		return err
	}
	if stat.IsDir() {
		return errors.New("cannot push directory, must provide chart archive (.tgz)")
	}

	meta, err := loader.Load(chartRef)
	if err != nil {
		return err
	}

	client := pusher.opts.registryClient
	if client == nil {
		c, err := pusher.newRegistryClient()
		if err != nil {
			return err
		}
		client = c
	}

	chartBytes, err := os.ReadFile(chartRef)
	if err != nil {
		return err
	}

	var pushOpts []registry.PushOption
	provRef := fmt.Sprintf("%s.prov", chartRef)
	if _, err := os.Stat(provRef); err == nil {
		provBytes, err := os.ReadFile(provRef)
		if err != nil {
			return err
		}
		pushOpts = append(pushOpts, registry.PushOptProvData(provBytes))
	}

	ref := fmt.Sprintf("%s:%s",
		path.Join(strings.TrimPrefix(href, fmt.Sprintf("%s://", registry.OCIScheme)), meta.Metadata.Name),
		meta.Metadata.Version)

	chartCreationTime := ctime.Created(stat)
	pushOpts = append(pushOpts, registry.PushOptCreationTime(chartCreationTime.Format(time.RFC3339)))

	_, err = client.Push(chartBytes, ref, pushOpts...)
	return err
}

// NewOCIPusher constructs a valid OCI client as a Pusher
func NewOCIPusher(ops ...Option) (Pusher, error) {
	var client OCIPusher

	for _, opt := range ops {
		opt(&client.opts)
	}

	return &client, nil
}

func (pusher *OCIPusher) newRegistryClient() (*registry.Client, error) {
	if (pusher.opts.certFile != "" && pusher.opts.keyFile != "") || pusher.opts.caFile != "" || pusher.opts.insecureSkipTLSverify {
		tlsConf, err := tlsutil.NewClientTLS(pusher.opts.certFile, pusher.opts.keyFile, pusher.opts.caFile, pusher.opts.insecureSkipTLSverify)
		if err != nil {
			return nil, errors.Wrap(err, "can't create TLS config for client")
		}

		registryClient, err := registry.NewClient(
			registry.ClientOptHTTPClient(&http.Client{
				// From https://github.com/google/go-containerregistry/blob/31786c6cbb82d6ec4fb8eb79cd9387905130534e/pkg/v1/remote/options.go#L87
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
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
					TLSClientConfig:       tlsConf,
				},
			}),
			registry.ClientOptEnableCache(true),
		)
		if err != nil {
			return nil, err
		}
		return registryClient, nil
	}

	opts := []registry.ClientOption{registry.ClientOptEnableCache(true)}
	if pusher.opts.plainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}
