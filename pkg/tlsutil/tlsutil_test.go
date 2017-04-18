/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"path/filepath"
	"testing"
)

const tlsTestDir = "../../testdata"

const (
	testCaCertFile = "ca.pem"
	testCertFile   = "crt.pem"
	testKeyFile    = "key.pem"
)

func TestClientConfig(t *testing.T) {
	opts := Options{
		CaCertFile:         testfile(t, testCaCertFile),
		CertFile:           testfile(t, testCertFile),
		KeyFile:            testfile(t, testKeyFile),
		InsecureSkipVerify: false,
	}

	cfg, err := ClientConfig(opts)
	if err != nil {
		t.Fatalf("error building tls client config: %v", err)
	}

	if got := len(cfg.Certificates); got != 1 {
		t.Fatalf("expecting 1 client certificates, got %d", got)
	}
	if cfg.InsecureSkipVerify {
		t.Fatalf("insecure skip verify mistmatch, expecting false")
	}
	if cfg.RootCAs == nil {
		t.Fatalf("mismatch tls RootCAs, expecting non-nil")
	}
}

func TestServerConfig(t *testing.T) {
	opts := Options{
		CaCertFile: testfile(t, testCaCertFile),
		CertFile:   testfile(t, testCertFile),
		KeyFile:    testfile(t, testKeyFile),
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	cfg, err := ServerConfig(opts)
	if err != nil {
		t.Fatalf("error building tls server config: %v", err)
	}
	if got := cfg.MinVersion; got != tls.VersionTLS12 {
		t.Errorf("expecting TLS version 1.2, got %d", got)
	}
	if got := cfg.ClientCAs; got == nil {
		t.Errorf("expecting non-nil CA pool")
	}
}

func testfile(t *testing.T, file string) (path string) {
	var err error
	if path, err = filepath.Abs(filepath.Join(tlsTestDir, file)); err != nil {
		t.Fatalf("error getting absolute path to test file %q: %v", file, err)
	}
	return path
}
