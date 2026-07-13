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
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/registry"
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
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: no such file", chartRef)
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
	provRef := chartRef + ".prov"
	if _, err := os.Stat(provRef); err == nil {
		provBytes, err := os.ReadFile(provRef)
		if err != nil {
			return err
		}
		pushOpts = append(pushOpts, registry.PushOptProvData(provBytes))
	}

	// Resolve the version used as the base of the OCI tag. With strict
	// versioning enabled this is the sanitized semver form of the chart
	// version; otherwise it is the raw chart version. The registry client
	// still applies its usual tag transformations (e.g. replacing plus (+)
	// signs with underscores) afterwards.
	version, err := resolveOCITagVersion(meta.Metadata.Version, pusher.opts.ociStrictVersion)
	if err != nil {
		return err
	}

	// The sanitized version may differ from the raw chart version, so relax
	// the registry client's strict-mode assertion (which requires the tag to
	// equal the raw chart version) when it does.
	if version != meta.Metadata.Version {
		pushOpts = append(pushOpts, registry.PushOptStrictMode(false))
	}

	ref := fmt.Sprintf("%s:%s",
		path.Join(strings.TrimPrefix(href, registry.OCIScheme+"://"), meta.Metadata.Name),
		version)

	// The time the chart was "created" is semantically the time the chart archive file was last written(modified)
	chartArchiveFileCreatedTime := stat.ModTime()
	pushOpts = append(pushOpts, registry.PushOptCreationTime(chartArchiveFileCreatedTime.Format(time.RFC3339)))

	_, err = client.Push(chartBytes, ref, pushOpts...)
	return err
}

// resolveOCITagVersion returns the version string to use as the base of the OCI
// tag for a chart. When ociStrictVersion is false the raw chart version is
// returned unchanged. When it is true the version is parsed with semver and its
// sanitized string representation is returned, so that a canonical semver tag is
// produced regardless of how the version was written in Chart.yaml.
func resolveOCITagVersion(rawVersion string, ociStrictVersion bool) (string, error) {
	if !ociStrictVersion {
		return rawVersion, nil
	}

	parsedVersion, err := semver.NewVersion(rawVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse chart version %q as semver: %w", rawVersion, err)
	}

	return parsedVersion.String(), nil
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
	if (pusher.opts.certFile != "" && pusher.opts.keyFile != "") || pusher.opts.caFile != "" || pusher.opts.insecureSkipTLSVerify {
		tlsConf, err := tlsutil.NewTLSConfig(
			tlsutil.WithInsecureSkipVerify(pusher.opts.insecureSkipTLSVerify),
			tlsutil.WithCertKeyPairFiles(pusher.opts.certFile, pusher.opts.keyFile),
			tlsutil.WithCAFile(pusher.opts.caFile),
		)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %w", err)
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
