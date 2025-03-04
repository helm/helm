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

package action

import (
	"io"

	"helm.sh/helm/v3/pkg/registry"
)

// RegistryLogin performs a registry login operation.
type RegistryLogin struct {
	cfg       *Configuration
	certFile  string
	keyFile   string
	caFile    string
	insecure  bool
	plainHTTP bool
}

type RegistryLoginOpt func(*RegistryLogin) error

// WithCertFile specifies the path to the certificate file to use for TLS.
func WithCertFile(certFile string) RegistryLoginOpt {
	return func(r *RegistryLogin) error {
		r.certFile = certFile
		return nil
	}
}

// WithInsecure specifies whether to verify certificates.
func WithInsecure(insecure bool) RegistryLoginOpt {
	return func(r *RegistryLogin) error {
		r.insecure = insecure
		return nil
	}
}

// WithKeyFile specifies the path to the key file to use for TLS.
func WithKeyFile(keyFile string) RegistryLoginOpt {
	return func(r *RegistryLogin) error {
		r.keyFile = keyFile
		return nil
	}
}

// WithCAFile specifies the path to the CA file to use for TLS.
func WithCAFile(caFile string) RegistryLoginOpt {
	return func(r *RegistryLogin) error {
		r.caFile = caFile
		return nil
	}
}

// WithPlainHTTPLogin use http rather than https for login.
func WithPlainHTTPLogin(isPlain bool) RegistryLoginOpt {
	return func(r *RegistryLogin) error {
		r.plainHTTP = isPlain
		return nil
	}
}

// NewRegistryLogin creates a new RegistryLogin object with the given configuration.
func NewRegistryLogin(cfg *Configuration) *RegistryLogin {
	return &RegistryLogin{
		cfg: cfg,
	}
}

// Run executes the registry login operation
func (a *RegistryLogin) Run(_ io.Writer, hostname string, username string, password string, opts ...RegistryLoginOpt) error {
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return err
		}
	}

	return a.cfg.RegistryClient.Login(
		hostname,
		registry.LoginOptBasicAuth(username, password),
		registry.LoginOptInsecure(a.insecure),
		registry.LoginOptTLSClientConfig(a.certFile, a.keyFile, a.caFile),
		registry.LoginOptPlainText(a.plainHTTP),
	)
}
