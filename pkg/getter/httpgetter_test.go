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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/internal/version"
	"helm.sh/helm/v4/pkg/cli"
)

func TestHTTPGetter(t *testing.T) {
	g, err := NewHTTPGetter(WithURL("http://example.com"))
	require.NoError(t, err)

	_, ok := g.(*HTTPGetter)
	require.True(t, ok, "Expected NewHTTPGetter to produce an *HTTPGetter")

	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	insecure := false
	timeout := time.Second * 5
	transport := &http.Transport{}

	// Test with getterOptions
	g, err = NewHTTPGetter(
		WithBasicAuth("I", "Am"),
		WithPassCredentialsAll(false),
		WithUserAgent("Groot"),
		WithTLSClientConfig(pub, priv, ca),
		WithInsecureSkipVerifyTLS(insecure),
		WithTimeout(timeout),
		WithTransport(transport),
	)
	require.NoError(t, err)

	hg, ok := g.(*HTTPGetter)
	require.True(t, ok, "expected NewHTTPGetter to produce an *HTTPGetter")
	assert.Equal(t, "I", hg.opts.username, "Expected NewHTTPGetter to contain %q as the username, got %q", "I", hg.opts.username)
	assert.Equal(t, "Am", hg.opts.password, "Expected NewHTTPGetter to contain %q as the password, got %q", "Am", hg.opts.password)
	assert.False(t, hg.opts.passCredentialsAll, "Expected NewHTTPGetter to contain %t as PassCredentialsAll, got %t", false, hg.opts.passCredentialsAll)
	assert.Equal(t, "Groot", hg.opts.userAgent, "Expected NewHTTPGetter to contain %q as the user agent, got %q", "Groot", hg.opts.userAgent)
	assert.Equal(t, pub, hg.opts.certFile, "Expected NewHTTPGetter to contain %q as the public key file, got %q", pub, hg.opts.certFile)
	assert.Equal(t, priv, hg.opts.keyFile, "Expected NewHTTPGetter to contain %q as the private key file, got %q", priv, hg.opts.keyFile)
	assert.Equal(t, ca, hg.opts.caFile, "Expected NewHTTPGetter to contain %q as the CA file, got %q", ca, hg.opts.caFile)
	assert.Equal(t, insecure, hg.opts.insecureSkipVerifyTLS, "Expected NewHTTPGetter to contain %t as InsecureSkipVerifyTLs flag, got %t", false, hg.opts.insecureSkipVerifyTLS)
	assert.Equal(t, timeout, hg.opts.timeout, "Expected NewHTTPGetter to contain %s as Timeout flag, got %s", timeout, hg.opts.timeout)
	assert.Same(t, transport, hg.opts.transport, "Expected NewHTTPGetter to contain %p as Transport, got %p", transport, hg.opts.transport)

	// Test if setting insecureSkipVerifyTLS is being passed to the ops
	insecure = true

	g, err = NewHTTPGetter(
		WithInsecureSkipVerifyTLS(insecure),
	)
	require.NoError(t, err)

	hg, ok = g.(*HTTPGetter)
	require.True(t, ok, "expected NewHTTPGetter to produce an *HTTPGetter")
	assert.Equal(t, insecure, hg.opts.insecureSkipVerifyTLS, "Expected NewHTTPGetter to contain %t as InsecureSkipVerifyTLs flag, got %t", insecure, hg.opts.insecureSkipVerifyTLS)

	// Checking false by default
	assert.False(t, hg.opts.passCredentialsAll, "Expected NewHTTPGetter to contain %t as PassCredentialsAll, got %t", false, hg.opts.passCredentialsAll)

	// Test setting PassCredentialsAll
	g, err = NewHTTPGetter(
		WithBasicAuth("I", "Am"),
		WithPassCredentialsAll(true),
	)
	require.NoError(t, err)

	hg, ok = g.(*HTTPGetter)
	require.True(t, ok, "expected NewHTTPGetter to produce an *HTTPGetter")
	assert.True(t, hg.opts.passCredentialsAll, "Expected NewHTTPGetter to contain %t as PassCredentialsAll, got %t", true, hg.opts.passCredentialsAll)
}

func TestDownload(t *testing.T) {
	expect := "Call me Ishmael"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultUserAgent := version.GetUserAgent()
		assert.Equal(t, defaultUserAgent, r.UserAgent(), "Expected '%s', got '%s'", defaultUserAgent, r.UserAgent())
		fmt.Fprint(w, expect)
	}))
	defer srv.Close()

	g, err := All(cli.New()).ByScheme("http")
	require.NoError(t, err)
	got, err := g.Get(srv.URL, WithURL(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, expect, got.String(), "Expected %q, got %q", expect, got.String())

	// test with http server
	const expectedUserAgent = "I am Groot"
	basicAuthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok, "Expected request to use basic auth")
		assert.Equal(t, "username", username)
		assert.Equal(t, "password", password)
		assert.Equal(t, expectedUserAgent, r.UserAgent(), "Expected '%s', got '%s'", expectedUserAgent, r.UserAgent())
		fmt.Fprint(w, expect)
	}))

	defer basicAuthSrv.Close()

	u, _ := url.ParseRequestURI(basicAuthSrv.URL)
	httpgetter, err := NewHTTPGetter(
		WithURL(u.String()),
		WithBasicAuth("username", "password"),
		WithPassCredentialsAll(false),
		WithUserAgent(expectedUserAgent),
	)
	require.NoError(t, err)
	got, err = httpgetter.Get(u.String())
	require.NoError(t, err)

	assert.Equal(t, expect, got.String(), "Expected %q, got %q", expect, got.String())

	// test with Get URL differing from withURL
	crossAuthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.False(t, ok)
		assert.NotEqual(t, "username", username)
		assert.NotEqual(t, "password", password)
		fmt.Fprint(w, expect)
	}))

	defer crossAuthSrv.Close()

	u, _ = url.ParseRequestURI(crossAuthSrv.URL)

	// A different host is provided for the WithURL from the one used for Get
	u2, _ := url.ParseRequestURI(crossAuthSrv.URL)
	host := strings.Split(u2.Host, ":")
	host[0] += "a"
	u2.Host = strings.Join(host, ":")
	httpgetter, err = NewHTTPGetter(
		WithURL(u2.String()),
		WithBasicAuth("username", "password"),
		WithPassCredentialsAll(false),
	)
	require.NoError(t, err)
	got, err = httpgetter.Get(u.String())
	require.NoError(t, err)

	assert.Equal(t, expect, got.String(), "Expected %q, got %q", expect, got.String())

	// test with Get URL differing from withURL and should pass creds
	crossAuthSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok, "Expected request to use basic auth")
		assert.Equal(t, "username", username)
		assert.Equal(t, "password", password)
		fmt.Fprint(w, expect)
	}))

	defer crossAuthSrv.Close()

	u, _ = url.ParseRequestURI(crossAuthSrv.URL)

	// A different host is provided for the WithURL from the one used for Get
	u2, _ = url.ParseRequestURI(crossAuthSrv.URL)
	host = strings.Split(u2.Host, ":")
	host[0] += "a"
	u2.Host = strings.Join(host, ":")
	httpgetter, err = NewHTTPGetter(
		WithURL(u2.String()),
		WithBasicAuth("username", "password"),
		WithPassCredentialsAll(true),
	)
	require.NoError(t, err)
	got, err = httpgetter.Get(u.String())
	require.NoError(t, err)

	assert.Equal(t, expect, got.String(), "Expected %q, got %q", expect, got.String())

	// test server with varied Accept Header
	const expectedAcceptHeader = "application/gzip,application/octet-stream"
	acceptHeaderSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedAcceptHeader, r.Header.Get("Accept"), "Expected '%s', got '%s'", expectedAcceptHeader, r.Header.Get("Accept"))
		fmt.Fprint(w, expect)
	}))

	defer acceptHeaderSrv.Close()

	u, _ = url.ParseRequestURI(acceptHeaderSrv.URL)
	httpgetter, err = NewHTTPGetter(
		WithAcceptHeader(expectedAcceptHeader),
	)
	require.NoError(t, err)
	_, err = httpgetter.Get(u.String())
	require.NoError(t, err)
}

func TestDownloadTLS(t *testing.T) {
	cd := "../../testdata"
	ca, pub, priv := filepath.Join(cd, "rootca.crt"), filepath.Join(cd, "crt.pem"), filepath.Join(cd, "key.pem")
	insecureSkipTLSVerify := false

	tlsSrv := httptest.NewUnstartedServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	tlsConf, err := tlsutil.NewTLSConfig(
		tlsutil.WithInsecureSkipVerify(insecureSkipTLSVerify),
		tlsutil.WithCertKeyPairFiles(pub, priv),
		tlsutil.WithCAFile(ca),
	)
	require.NoError(t, err, "can't create TLS config for client")
	tlsConf.ServerName = "helm.sh"
	tlsSrv.TLS = tlsConf
	tlsSrv.StartTLS()
	defer tlsSrv.Close()

	u, _ := url.ParseRequestURI(tlsSrv.URL)
	g, err := NewHTTPGetter(
		WithURL(u.String()),
		WithTLSClientConfig(pub, priv, ca),
	)
	require.NoError(t, err)

	_, err = g.Get(u.String())
	require.NoError(t, err)

	// now test with TLS config being passed along in .Get (see #6635)
	g, err = NewHTTPGetter()
	require.NoError(t, err)

	_, err = g.Get(u.String(), WithURL(u.String()), WithTLSClientConfig(pub, priv, ca))
	require.NoError(t, err)

	// test with only the CA file (see also #6635)
	g, err = NewHTTPGetter()
	require.NoError(t, err)

	_, err = g.Get(u.String(), WithURL(u.String()), WithTLSClientConfig("", "", ca))
	assert.NoError(t, err)
}

func TestDownloadTLSWithRedirect(t *testing.T) {
	cd := "../../testdata"
	srv2Resp := "hello"
	insecureSkipTLSVerify := false

	// Server 2 that will actually fulfil the request.
	ca, pub, priv := filepath.Join(cd, "rootca.crt"), filepath.Join(cd, "localhost-crt.pem"), filepath.Join(cd, "key.pem")
	tlsConf, err := tlsutil.NewTLSConfig(
		tlsutil.WithCAFile(ca),
		tlsutil.WithCertKeyPairFiles(pub, priv),
		tlsutil.WithInsecureSkipVerify(insecureSkipTLSVerify),
	)

	require.NoError(t, err, "can't create TLS config for client")

	tlsSrv2 := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/plain")
		rw.Write([]byte(srv2Resp))
	}))

	tlsSrv2.TLS = tlsConf
	tlsSrv2.StartTLS()
	defer tlsSrv2.Close()

	// Server 1 responds with a redirect to Server 2.
	ca, pub, priv = filepath.Join(cd, "rootca.crt"), filepath.Join(cd, "crt.pem"), filepath.Join(cd, "key.pem")
	tlsConf, err = tlsutil.NewTLSConfig(
		tlsutil.WithCAFile(ca),
		tlsutil.WithCertKeyPairFiles(pub, priv),
		tlsutil.WithInsecureSkipVerify(insecureSkipTLSVerify),
	)

	require.NoError(t, err, "can't create TLS config for client")

	tlsSrv1 := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		u, _ := url.ParseRequestURI(tlsSrv2.URL)

		// Make the request using the hostname 'localhost' (to which 'localhost-crt.pem' is issued)
		// to verify that a successful TLS connection is made even if the client doesn't specify
		// the hostname (SNI) in `tls.Config.ServerName`. By default the hostname is derived from the
		// request URL for every request (including redirects). Setting `tls.Config.ServerName` on the
		// client just overrides the remote endpoint's hostname.
		// See https://github.com/golang/go/blob/3979fb9/src/net/http/transport.go#L1505-L1513.
		u.Host = "localhost:" + u.Port()

		http.Redirect(rw, r, u.String(), http.StatusTemporaryRedirect)
	}))

	tlsSrv1.TLS = tlsConf
	tlsSrv1.StartTLS()
	defer tlsSrv1.Close()

	u, _ := url.ParseRequestURI(tlsSrv1.URL)

	t.Run("Test with TLS", func(t *testing.T) {
		g, err := NewHTTPGetter(
			WithURL(u.String()),
			WithTLSClientConfig(pub, priv, ca),
		)
		require.NoError(t, err)

		buf, err := g.Get(u.String())
		require.NoError(t, err)

		b, err := io.ReadAll(buf)
		require.NoError(t, err)

		assert.Equal(t, srv2Resp, string(b), "expected response from Server2 to be '%s', instead got: %s", srv2Resp, string(b))
	})

	t.Run("Test with TLS config being passed along in .Get (see #6635)", func(t *testing.T) {
		g, err := NewHTTPGetter()
		require.NoError(t, err)

		buf, err := g.Get(u.String(), WithURL(u.String()), WithTLSClientConfig(pub, priv, ca))
		require.NoError(t, err)

		b, err := io.ReadAll(buf)
		require.NoError(t, err)

		assert.Equal(t, srv2Resp, string(b), "expected response from Server2 to be '%s', instead got: %s", srv2Resp, string(b))
	})

	t.Run("Test with only the CA file (see also #6635)", func(t *testing.T) {
		g, err := NewHTTPGetter()
		require.NoError(t, err)

		buf, err := g.Get(u.String(), WithURL(u.String()), WithTLSClientConfig("", "", ca))
		require.NoError(t, err)

		b, err := io.ReadAll(buf)
		require.NoError(t, err)

		assert.Equal(t, srv2Resp, string(b), "expected response from Server2 to be '%s', instead got: %s", srv2Resp, string(b))
	})
}

func TestDownloadInsecureSkipTLSVerify(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer ts.Close()

	u, _ := url.ParseRequestURI(ts.URL)

	// Ensure the default behavior did not change
	g, err := NewHTTPGetter(
		WithURL(u.String()),
	)
	require.NoError(t, err)

	_, err = g.Get(u.String())
	require.Errorf(t, err, "Expected Getter to throw an error")

	// Test certificate check skip
	g, err = NewHTTPGetter(
		WithURL(u.String()),
		WithInsecureSkipVerifyTLS(true),
	)
	require.NoError(t, err)
	_, err = g.Get(u.String())
	assert.NoError(t, err)
}

func TestHTTPGetterTarDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, _ := os.Open("testdata/empty-0.0.1.tgz")
		defer f.Close()

		b := make([]byte, 512)
		f.Read(b)
		// Get the file size
		FileStat, _ := f.Stat()
		FileSize := strconv.FormatInt(FileStat.Size(), 10)

		// Simulating improper header values from bitbucket
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", FileSize)

		f.Seek(0, 0)
		io.Copy(w, f)
	}))

	defer srv.Close()

	g, err := NewHTTPGetter(WithURL(srv.URL))
	require.NoError(t, err)

	data, _ := g.Get(srv.URL)
	mimeType := http.DetectContentType(data.Bytes())

	expectedMimeType := "application/x-gzip"
	require.Equal(t, expectedMimeType, mimeType, "Expected response with MIME type %s, but got %s", expectedMimeType, mimeType)
}

func TestHttpClientInsecureSkipVerify(t *testing.T) {
	g := HTTPGetter{}
	g.opts.url = "https://localhost"
	verifyInsecureSkipVerify(t, &g, "Blank HTTPGetter", false)

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.caFile = "testdata/ca.crt"
	verifyInsecureSkipVerify(t, &g, "HTTPGetter with ca file", false)

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.insecureSkipVerifyTLS = true
	verifyInsecureSkipVerify(t, &g, "HTTPGetter with skip cert verification only", true)

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.certFile = "testdata/client.crt"
	g.opts.keyFile = "testdata/client.key"
	g.opts.insecureSkipVerifyTLS = true
	transport := verifyInsecureSkipVerify(t, &g, "HTTPGetter with 2 way ssl", true)
	require.NotEmpty(t, transport.TLSClientConfig.Certificates, "transport.TLSClientConfig.Certificates is not present")
}

func verifyInsecureSkipVerify(t *testing.T, g *HTTPGetter, caseName string, expectedValue bool) *http.Transport {
	t.Helper()
	returnVal, err := g.httpClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, returnVal, "Expected non nil value for http client")
	transport := (returnVal.Transport).(*http.Transport)
	gotValue := false
	if transport.TLSClientConfig != nil {
		gotValue = transport.TLSClientConfig.InsecureSkipVerify
	}
	require.Equal(t, expectedValue, gotValue, "Case Name = %s\nInsecureSkipVerify did not come as expected. Expected = %t; Got = %v", caseName, expectedValue, gotValue)
	return transport
}

func TestDefaultHTTPTransportReuse(t *testing.T) {
	g := HTTPGetter{}

	httpClient1, err := g.httpClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, httpClient1, "Expected non nil value for http client")

	transport1 := (httpClient1.Transport).(*http.Transport)

	httpClient2, err := g.httpClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, httpClient2, "Expected non nil value for http client")

	transport2 := (httpClient2.Transport).(*http.Transport)

	require.Equal(t, transport2, transport1, "Expected default transport to be reused")
}

func TestHTTPTransportOption(t *testing.T) {
	transport := &http.Transport{}

	g := HTTPGetter{}
	g.opts.transport = transport
	httpClient1, err := g.httpClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, httpClient1, "Expected non nil value for http client")

	transport1 := (httpClient1.Transport).(*http.Transport)

	require.Equal(t, transport, transport1, "Expected transport option to be applied")

	httpClient2, err := g.httpClient(g.opts)

	require.NoError(t, err)

	require.NotNil(t, httpClient2, "Expected non nil value for http client")

	transport2 := (httpClient2.Transport).(*http.Transport)

	require.Equal(t, transport2, transport1, "Expected applied transport to be reused")

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.certFile = "testdata/client.crt"
	g.opts.keyFile = "testdata/client.key"
	g.opts.insecureSkipVerifyTLS = true
	g.opts.transport = transport
	usedTransport := verifyInsecureSkipVerify(t, &g, "HTTPGetter with 2 way ssl", false)
	require.Nil(t, usedTransport.TLSClientConfig, "transport.TLSClientConfig should not be set")
}
