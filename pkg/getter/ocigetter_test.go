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
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/registry"
)

func TestOCIGetter(t *testing.T) {
	g, err := NewOCIGetter(WithURL("oci://example.com"))
	require.NoError(t, err)

	_, ok := g.(*OCIGetter)
	require.True(t, ok, "Expected NewOCIGetter to produce an *OCIGetter")

	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	timeout := time.Second * 5
	transport := &http.Transport{}
	insecureSkipVerifyTLS := false
	plainHTTP := false

	// Test with getterOptions
	g, err = NewOCIGetter(
		WithBasicAuth("I", "Am"),
		WithTLSClientConfig(pub, priv, ca),
		WithTimeout(timeout),
		WithTransport(transport),
		WithInsecureSkipVerifyTLS(insecureSkipVerifyTLS),
		WithPlainHTTP(plainHTTP),
	)
	require.NoError(t, err)

	og, ok := g.(*OCIGetter)
	require.True(t, ok, "expected NewOCIGetter to produce an *OCIGetter")
	assert.Equal(t, "I", og.opts.username, "Expected NewOCIGetter to contain %q as the username, got %q", "I", og.opts.username)
	assert.Equal(t, "Am", og.opts.password, "Expected NewOCIGetter to contain %q as the password, got %q", "Am", og.opts.password)
	assert.Equal(t, pub, og.opts.certFile, "Expected NewOCIGetter to contain %q as the public key file, got %q", pub, og.opts.certFile)
	assert.Equal(t, priv, og.opts.keyFile, "Expected NewOCIGetter to contain %q as the private key file, got %q", priv, og.opts.keyFile)
	assert.Equal(t, ca, og.opts.caFile, "Expected NewOCIGetter to contain %q as the CA file, got %q", ca, og.opts.caFile)
	assert.Equal(t, timeout, og.opts.timeout, "Expected NewOCIGetter to contain %s as Timeout flag, got %s", timeout, og.opts.timeout)
	assert.Equal(t, transport, og.opts.transport, "Expected NewOCIGetter to contain %p as Transport, got %p", transport, og.opts.transport)
	assert.Equal(t, plainHTTP, og.opts.plainHTTP, "Expected NewOCIGetter to have plainHTTP as %t, got %t", plainHTTP, og.opts.plainHTTP)
	assert.Equal(t, insecureSkipVerifyTLS, og.opts.insecureSkipVerifyTLS, "Expected NewOCIGetter to have insecureSkipVerifyTLS as %t, got %t", insecureSkipVerifyTLS, og.opts.insecureSkipVerifyTLS)

	// Test if setting registryClient is being passed to the ops
	registryClient, err := registry.NewClient()
	require.NoError(t, err)

	g, err = NewOCIGetter(
		WithRegistryClient(registryClient),
	)
	require.NoError(t, err)
	og, ok = g.(*OCIGetter)
	require.True(t, ok, "expected NewOCIGetter to produce an *OCIGetter")
	assert.Equal(t, registryClient, og.opts.registryClient, "Expected NewOCIGetter to contain %p as RegistryClient, got %p", registryClient, og.opts.registryClient)
}

func TestOCIHTTPTransportReuse(t *testing.T) {
	g := OCIGetter{}

	_, err := g.newRegistryClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, g.transport, "Expected non nil value for transport")

	transport1 := g.transport

	_, err = g.newRegistryClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, g.transport, "Expected non nil value for transport")

	transport2 := g.transport

	require.Equal(t, transport2, transport1, "Expected default transport to be reused")
}

func TestOCIGetterPerCallOptionsDoNotMutateState(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	baseURL := "oci://" + srv.Listener.Addr().String()
	ref := baseURL + "/ns/chart:0.1.0"

	g, err := NewOCIGetter(WithURL(baseURL))
	require.NoError(t, err)

	og, ok := g.(*OCIGetter)
	require.True(t, ok)

	// Initialize the shared default transport; connecting to the self-signed
	// test server without InsecureSkipVerify must fail TLS verification.
	_, err = og.Get(ref)
	var unknownAuthErr x509.UnknownAuthorityError
	require.ErrorAs(t, err, &unknownAuthErr)
	defaultTransport := og.transport
	require.NotNil(t, defaultTransport)

	// A single call with per-call overrides must not mutate receiver options
	// or the shared default transport's TLS configuration.
	_, err = og.Get(
		baseURL+"/ns/chart",
		WithInsecureSkipVerifyTLS(true),
		WithTagName("1.2.3"),
		WithTimeout(9*time.Second),
	)
	require.Error(t, err)

	assert.False(t, og.opts.insecureSkipVerifyTLS)
	assert.Empty(t, og.opts.version)
	assert.Zero(t, og.opts.timeout)
	assert.False(t, defaultTransport.TLSClientConfig.InsecureSkipVerify)

	// Subsequent calls without WithInsecureSkipVerifyTLS(true) must still
	// verify TLS certificates rather than inheriting the earlier override.
	_, err = og.Get(ref)
	require.ErrorAs(t, err, &unknownAuthErr)
}

func TestOCIGetterConcurrentGet(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	baseURL := "oci://" + srv.Listener.Addr().String()
	ref := baseURL + "/ns/chart"

	g, err := NewOCIGetter(WithURL(baseURL))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			if i%2 == 0 {
				_, _ = g.Get(ref+":0.1.0", WithInsecureSkipVerifyTLS(true), WithTimeout(2*time.Second))
				return
			}
			_, _ = g.Get(ref, WithTagName("0.1.0"))
		})
	}
	wg.Wait()
}
