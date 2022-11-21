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
	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

// options are generic parameters to be provided to the pusher during instantiation.
//
// Pushers may or may not ignore these parameters as they are passed in.
type options struct {
	registryClient        *registry.Client
	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSverify bool
}

// Option allows specifying various settings configurable by the user for overriding the defaults
// used when performing Push operations with the Pusher.
type Option func(*options)

// WithRegistryClient sets the registryClient option.
func WithRegistryClient(client *registry.Client) Option {
	return func(opts *options) {
		opts.registryClient = client
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

// WithInsecureSkipTLSVerify determines if a TLS Certificate will be checked
func WithInsecureSkipTLSVerify(insecureSkipTLSVerify bool) Option {
	return func(opts *options) {
		opts.insecureSkipTLSverify = insecureSkipTLSVerify
	}
}

// Pusher is an interface to support upload to the specified URL.
type Pusher interface {
	// Push file content by url string
	Push(chartRef, url string, options ...Option) error
}

// Constructor is the function for every pusher which creates a specific instance
// according to the configuration
type Constructor func(options ...Option) (Pusher, error)

// Provider represents any pusher and the schemes that it supports.
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
func (p Providers) ByScheme(scheme string) (Pusher, error) {
	for _, pp := range p {
		if pp.Provides(scheme) {
			return pp.New()
		}
	}
	return nil, errors.Errorf("scheme %q not supported", scheme)
}

var ociProvider = Provider{
	Schemes: []string{registry.OCIScheme},
	New:     NewOCIPusher,
}

// All finds all of the registered pushers as a list of Provider instances.
// Currently, just the built-in pushers are collected.
func All(settings *cli.EnvSettings) Providers {
	result := Providers{ociProvider}
	return result
}
