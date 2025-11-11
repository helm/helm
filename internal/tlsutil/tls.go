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

package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"errors"
)

type TLSConfigOptions struct {
	insecureSkipTLSVerify     bool
	certPEMBlock, keyPEMBlock []byte
	caPEMBlock                []byte
}

type TLSConfigOption func(options *TLSConfigOptions) error

func WithInsecureSkipVerify(insecureSkipTLSVerify bool) TLSConfigOption {
	return func(options *TLSConfigOptions) error {
		options.insecureSkipTLSVerify = insecureSkipTLSVerify

		return nil
	}
}

func WithCertKeyPairFiles(certFile, keyFile string) TLSConfigOption {
	return func(options *TLSConfigOptions) error {
		if certFile == "" && keyFile == "" {
			return nil
		}

		certPEMBlock, err := os.ReadFile(certFile)
		if err != nil {
			return fmt.Errorf("unable to read cert file: %q: %w", certFile, err)
		}

		keyPEMBlock, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("unable to read key file: %q: %w", keyFile, err)
		}

		options.certPEMBlock = certPEMBlock
		options.keyPEMBlock = keyPEMBlock

		return nil
	}
}

func WithCAFile(caFile string) TLSConfigOption {
	return func(options *TLSConfigOptions) error {
		if caFile == "" {
			return nil
		}

		caPEMBlock, err := os.ReadFile(caFile)
		if err != nil {
			return fmt.Errorf("can't read CA file: %q: %w", caFile, err)
		}

		options.caPEMBlock = caPEMBlock

		return nil
	}
}

func NewTLSConfig(options ...TLSConfigOption) (*tls.Config, error) {
	to := TLSConfigOptions{}

	errs := []error{}
	for _, option := range options {
		err := option(&to)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	config := tls.Config{
		InsecureSkipVerify: to.insecureSkipTLSVerify,
	}

	if len(to.certPEMBlock) > 0 && len(to.keyPEMBlock) > 0 {
		cert, err := tls.X509KeyPair(to.certPEMBlock, to.keyPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("unable to load cert from key pair: %w", err)
		}

		config.Certificates = []tls.Certificate{cert}
	}

	if len(to.caPEMBlock) > 0 {
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(to.caPEMBlock) {
			return nil, fmt.Errorf("failed to append certificates from pem block")
		}

		config.RootCAs = cp
	}

	return &config, nil
}
