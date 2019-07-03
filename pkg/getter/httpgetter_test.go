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

	"helm.sh/helm/internal/test"
)

func TestHTTPGetter(t *testing.T) {
	g, err := newHTTPGetter(WithURL("http://example.com"))
	if err != nil {
		t.Fatal(err)
	}

	if hg, ok := g.(*HTTPGetter); !ok {
		t.Fatal("Expected newHTTPGetter to produce an httpGetter")
	} else if hg.client != http.DefaultClient {
		t.Fatal("Expected newHTTPGetter to return a default HTTP client.")
	}

	// Test with SSL:
	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "ca.pem"), join(cd, "crt.pem"), join(cd, "key.pem")
	g, err = newHTTPGetter(
		WithURL("http://example.com"),
		WithTLSClientConfig(pub, priv, ca),
	)
	if err != nil {
		t.Fatal(err)
	}

	hg, ok := g.(*HTTPGetter)
	if !ok {
		t.Fatal("Expected newHTTPGetter to produce an httpGetter")
	}

	transport, ok := hg.client.Transport.(*http.Transport)
	if !ok {
		t.Errorf("Expected newHTTPGetter to set up an HTTP transport")
	}

	test.AssertGoldenString(t, transport.TLSClientConfig.ServerName, "output/httpgetter-servername.txt")

	// Test other options
	hg, err = NewHTTPGetter(
		WithBasicAuth("I", "Am"),
		WithUserAgent("Groot"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if hg.opts.username != "I" {
		t.Errorf("Expected NewHTTPGetter to contain %q as the username, got %q", "I", hg.opts.username)
	}

	if hg.opts.password != "Am" {
		t.Errorf("Expected NewHTTPGetter to contain %q as the password, got %q", "Am", hg.opts.password)
	}

	if hg.opts.userAgent != "Groot" {
		t.Errorf("Expected NewHTTPGetter to contain %q as the user agent, got %q", "Groot", hg.opts.userAgent)
	}
}
