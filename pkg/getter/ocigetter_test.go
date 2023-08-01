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
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/registry"
)

func TestOCIGetter(t *testing.T) {
	g, err := NewOCIGetter(WithURL("oci://example.com"))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := g.(*OCIGetter); !ok {
		t.Fatal("Expected NewOCIGetter to produce an *OCIGetter")
	}

	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	timeout := time.Second * 5
	transport := &http.Transport{}
	insecureSkipVerifyTLS := false
	plainHTTP := false

	// Test with options
	g, err = NewOCIGetter(
		WithBasicAuth("I", "Am"),
		WithTLSClientConfig(pub, priv, ca),
		WithTimeout(timeout),
		WithTransport(transport),
		WithInsecureSkipVerifyTLS(insecureSkipVerifyTLS),
		WithPlainHTTP(plainHTTP),
	)
	if err != nil {
		t.Fatal(err)
	}

	og, ok := g.(*OCIGetter)
	if !ok {
		t.Fatal("expected NewOCIGetter to produce an *OCIGetter")
	}

	if og.opts.username != "I" {
		t.Errorf("Expected NewOCIGetter to contain %q as the username, got %q", "I", og.opts.username)
	}

	if og.opts.password != "Am" {
		t.Errorf("Expected NewOCIGetter to contain %q as the password, got %q", "Am", og.opts.password)
	}

	if og.opts.certFile != pub {
		t.Errorf("Expected NewOCIGetter to contain %q as the public key file, got %q", pub, og.opts.certFile)
	}

	if og.opts.keyFile != priv {
		t.Errorf("Expected NewOCIGetter to contain %q as the private key file, got %q", priv, og.opts.keyFile)
	}

	if og.opts.caFile != ca {
		t.Errorf("Expected NewOCIGetter to contain %q as the CA file, got %q", ca, og.opts.caFile)
	}

	if og.opts.timeout != timeout {
		t.Errorf("Expected NewOCIGetter to contain %s as Timeout flag, got %s", timeout, og.opts.timeout)
	}

	if og.opts.transport != transport {
		t.Errorf("Expected NewOCIGetter to contain %p as Transport, got %p", transport, og.opts.transport)
	}

	if og.opts.plainHTTP != plainHTTP {
		t.Errorf("Expected NewOCIGetter to have plainHTTP as %t, got %t", plainHTTP, og.opts.plainHTTP)
	}

	if og.opts.insecureSkipVerifyTLS != insecureSkipVerifyTLS {
		t.Errorf("Expected NewOCIGetter to have insecureSkipVerifyTLS as %t, got %t", insecureSkipVerifyTLS, og.opts.insecureSkipVerifyTLS)
	}

	// Test if setting registryClient is being passed to the ops
	registryClient, err := registry.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	g, err = NewOCIGetter(
		WithRegistryClient(registryClient),
	)
	if err != nil {
		t.Fatal(err)
	}
	og, ok = g.(*OCIGetter)
	if !ok {
		t.Fatal("expected NewOCIGetter to produce an *OCIGetter")
	}

	if og.opts.registryClient != registryClient {
		t.Errorf("Expected NewOCIGetter to contain %p as RegistryClient, got %p", registryClient, og.opts.registryClient)
	}
}

func TestOCIHTTPTransportReuse(t *testing.T) {
	g := OCIGetter{}

	_, err := g.newRegistryClient()

	if err != nil {
		t.Fatal(err)
	}

	if g.transport == nil {
		t.Fatalf("Expected non nil value for transport")
	}

	transport1 := g.transport

	_, err = g.newRegistryClient()

	if err != nil {
		t.Fatal(err)
	}

	if g.transport == nil {
		t.Fatalf("Expected non nil value for transport")
	}

	transport2 := g.transport

	if transport1 != transport2 {
		t.Fatalf("Expected default transport to be reused")
	}
}
