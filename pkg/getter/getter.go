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
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/cli"
)

// options are generic parameters to be provided to the getter during instantiation.
//
// Getters may or may not ignore these parameters as they are passed in.
type options struct {
	url                   string
	certFile              string
	keyFile               string
	caFile                string
	unTar                 bool
	insecureSkipVerifyTLS bool
	username              string
	password              string
	passCredentialsAll    bool
	userAgent             string
	version               string
	registryClient        *registry.Client
	timeout               time.Duration
}

// Option allows specifying various settings configurable by the user for overriding the defaults
// used when performing Get operations with the Getter.
type Option func(*options)

// WithURL informs the getter the server name that will be used when fetching objects. Used in conjunction with
// WithTLSClientConfig to set the TLSClientConfig's server name.
func WithURL(url string) Option {
	return func(opts *options) {
		opts.url = url
	}
}

// WithBasicAuth sets the request's Authorization header to use the provided credentials
func WithBasicAuth(username, password string) Option {
	return func(opts *options) {
		opts.username = username
		opts.password = password
	}
}

func WithPassCredentialsAll(pass bool) Option {
	return func(opts *options) {
		opts.passCredentialsAll = pass
	}
}

// WithUserAgent sets the request's User-Agent header to use the provided agent name.
func WithUserAgent(userAgent string) Option {
	return func(opts *options) {
		opts.userAgent = userAgent
	}
}

// WithInsecureSkipVerifyTLS determines if a TLS Certificate will be checked
func WithInsecureSkipVerifyTLS(insecureSkipVerifyTLS bool) Option {
	return func(opts *options) {
		opts.insecureSkipVerifyTLS = insecureSkipVerifyTLS
	}
}

// WithTLSClientConfig sets the client auth with the provided credentials.
func WithTLSClientConfig(certFile, keyFile, caFile string) Option {
	return func(opts *options) {
		opts.certFile = certFile
		opts.keyFile = keyFile
		opts.caFile = caFile
	}
}

// WithTimeout sets the timeout for requests
func WithTimeout(timeout time.Duration) Option {
	return func(opts *options) {
		opts.timeout = timeout
	}
}

func WithTagName(tagname string) Option {
	return func(opts *options) {
		opts.version = tagname
	}
}

func WithRegistryClient(client *registry.Client) Option {
	return func(opts *options) {
		opts.registryClient = client
	}
}

func WithUntar() Option {
	return func(opts *options) {
		opts.unTar = true
	}
}

// Getter is an interface to support GET to the specified URL.
type Getter interface {
	// Get file content by url string
	Get(url string, options ...Option) (*bytes.Buffer, error)
}

// Constructor is the function for every getter which creates a specific instance
// according to the configuration
type Constructor func(options ...Option) (Getter, error)

// Provider represents any getter and the schemes that it supports.
//
// For example, an HTTP provider may provide one getter that handles both
// 'http' and 'https' schemes.
type Provider struct {
	Schemes []string
	New     Constructor
}

// Provides returns true if the given scheme is supported by this Provider.
func (p Provider) Provides(scheme string) bool {
	for _, i := range p.Schemes {
		if i == scheme {
			return true
		}
	}
	return false
}

// Providers is a collection of Provider objects.
type Providers []Provider

// ByScheme returns a Provider that handles the given scheme.
//
// If no provider handles this scheme, this will return an error.
func (p Providers) ByScheme(scheme string) (Getter, error) {
	for _, pp := range p {
		if pp.Provides(scheme) {
			return pp.New()
		}
	}
	return nil, errors.Errorf("scheme %q not supported", scheme)
}

var httpProvider = Provider{
	Schemes: []string{"http", "https"},
	New:     NewHTTPGetter,
}

var ociProvider = Provider{
	Schemes: []string{"oci"},
	New:     NewOCIGetter,
}

// All finds all of the registered getters as a list of Provider instances.
// Currently, the built-in getters and the discovered plugins with downloader
// notations are collected.
func All(settings *cli.EnvSettings) Providers {
	result := Providers{httpProvider, ociProvider}
	pluginDownloaders, _ := collectPlugins(settings)
	result = append(result, pluginDownloaders...)
	return result
}
