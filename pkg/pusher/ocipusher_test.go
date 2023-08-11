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
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/registry"
)

func TestNewOCIPusher(t *testing.T) {
	p, err := NewOCIPusher()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := p.(*OCIPusher); !ok {
		t.Fatal("Expected NewOCIPusher to produce an *OCIPusher")
	}

	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	insecureSkipTLSverify := false
	plainHTTP := false

	// Test with options
	p, err = NewOCIPusher(
		WithTLSClientConfig(pub, priv, ca),
		WithInsecureSkipTLSVerify(insecureSkipTLSverify),
		WithPlainHTTP(plainHTTP),
	)
	if err != nil {
		t.Fatal(err)
	}

	op, ok := p.(*OCIPusher)
	if !ok {
		t.Fatal("Expected NewOCIPusher to produce an *OCIPusher")
	}

	if op.opts.certFile != pub {
		t.Errorf("Expected NewOCIPusher to contain %q as the public key file, got %q", pub, op.opts.certFile)
	}

	if op.opts.keyFile != priv {
		t.Errorf("Expected NewOCIPusher to contain %q as the private key file, got %q", priv, op.opts.keyFile)
	}

	if op.opts.caFile != ca {
		t.Errorf("Expected NewOCIPusher to contain %q as the CA file, got %q", ca, op.opts.caFile)
	}

	if op.opts.plainHTTP != plainHTTP {
		t.Errorf("Expected NewOCIPusher to have plainHTTP as %t, got %t", plainHTTP, op.opts.plainHTTP)
	}

	if op.opts.insecureSkipTLSverify != insecureSkipTLSverify {
		t.Errorf("Expected NewOCIPusher to have insecureSkipVerifyTLS as %t, got %t", insecureSkipTLSverify, op.opts.insecureSkipTLSverify)
	}

	// Test if setting registryClient is being passed to the ops
	registryClient, err := registry.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	p, err = NewOCIPusher(
		WithRegistryClient(registryClient),
	)
	if err != nil {
		t.Fatal(err)
	}
	op, ok = p.(*OCIPusher)
	if !ok {
		t.Fatal("expected NewOCIPusher to produce an *OCIPusher")
	}

	if op.opts.registryClient != registryClient {
		t.Errorf("Expected NewOCIPusher to contain %p as RegistryClient, got %p", registryClient, op.opts.registryClient)
	}
}
