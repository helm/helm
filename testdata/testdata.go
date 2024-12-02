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

package testdata

import (
	"crypto/tls"
	"crypto/x509"
	"embed"

	"github.com/pkg/errors"
)

//go:embed rootca.crt rootca.key crt.pem key.pem
var tlsFiles embed.FS

func ReadTLSConfig(insecureSkipTLSverify bool) (*tls.Config, error) {
	config := tls.Config{
		InsecureSkipVerify: insecureSkipTLSverify,
	}

	certFile := "crt.pem"
	keyFile := "key.pem"
	caFile := "rootca.crt"

	certPEMBlock, err := tlsFiles.ReadFile(certFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read cert file: file=%q", certFile)
	}

	keyPEMBlock, err := tlsFiles.ReadFile(keyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read key file: file=%q", keyFile)
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	config.Certificates = []tls.Certificate{cert}

	tlsFiles.ReadFile("rootca.crt")

	b, err := tlsFiles.ReadFile(caFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read CA file: caFile=%q", caFile)
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(b) {
		return nil, errors.Wrapf(err, "failed to append certificates from file: caFile=%q", caFile)
	}

	config.RootCAs = cp

	return &config, nil
}
