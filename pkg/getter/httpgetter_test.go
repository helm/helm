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

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/tlsutil"
	"helm.sh/helm/v3/internal/version"
	"helm.sh/helm/v3/pkg/cli"
)

func TestHTTPGetter(t *testing.T) {
	g, err := NewHTTPGetter(WithURL("http://example.com"))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := g.(*HTTPGetter); !ok {
		t.Fatal("Expected NewHTTPGetter to produce an *HTTPGetter")
	}

	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	insecure := false
	timeout := time.Second * 5

	// Test with options
	g, err = NewHTTPGetter(
		WithBasicAuth("I", "Am"),
		WithPassCredentialsAll(false),
		WithUserAgent("Groot"),
		WithTLSClientConfig(pub, priv, ca),
		WithInsecureSkipVerifyTLS(insecure),
		WithTimeout(timeout),
	)
	if err != nil {
		t.Fatal(err)
	}

	hg, ok := g.(*HTTPGetter)
	if !ok {
		t.Fatal("expected NewHTTPGetter to produce an *HTTPGetter")
	}

	if hg.opts.username != "I" {
		t.Errorf("Expected NewHTTPGetter to contain %q as the username, got %q", "I", hg.opts.username)
	}

	if hg.opts.password != "Am" {
		t.Errorf("Expected NewHTTPGetter to contain %q as the password, got %q", "Am", hg.opts.password)
	}

	if hg.opts.passCredentialsAll != false {
		t.Errorf("Expected NewHTTPGetter to contain %t as PassCredentialsAll, got %t", false, hg.opts.passCredentialsAll)
	}

	if hg.opts.userAgent != "Groot" {
		t.Errorf("Expected NewHTTPGetter to contain %q as the user agent, got %q", "Groot", hg.opts.userAgent)
	}

	if hg.opts.certFile != pub {
		t.Errorf("Expected NewHTTPGetter to contain %q as the public key file, got %q", pub, hg.opts.certFile)
	}

	if hg.opts.keyFile != priv {
		t.Errorf("Expected NewHTTPGetter to contain %q as the private key file, got %q", priv, hg.opts.keyFile)
	}

	if hg.opts.caFile != ca {
		t.Errorf("Expected NewHTTPGetter to contain %q as the CA file, got %q", ca, hg.opts.caFile)
	}

	if hg.opts.insecureSkipVerifyTLS != insecure {
		t.Errorf("Expected NewHTTPGetter to contain %t as InsecureSkipVerifyTLs flag, got %t", false, hg.opts.insecureSkipVerifyTLS)
	}

	if hg.opts.timeout != timeout {
		t.Errorf("Expected NewHTTPGetter to contain %s as Timeout flag, got %s", timeout, hg.opts.timeout)
	}

	// Test if setting insecureSkipVerifyTLS is being passed to the ops
	insecure = true

	g, err = NewHTTPGetter(
		WithInsecureSkipVerifyTLS(insecure),
	)
	if err != nil {
		t.Fatal(err)
	}

	hg, ok = g.(*HTTPGetter)
	if !ok {
		t.Fatal("expected NewHTTPGetter to produce an *HTTPGetter")
	}

	if hg.opts.insecureSkipVerifyTLS != insecure {
		t.Errorf("Expected NewHTTPGetter to contain %t as InsecureSkipVerifyTLs flag, got %t", insecure, hg.opts.insecureSkipVerifyTLS)
	}

	// Checking false by default
	if hg.opts.passCredentialsAll != false {
		t.Errorf("Expected NewHTTPGetter to contain %t as PassCredentialsAll, got %t", false, hg.opts.passCredentialsAll)
	}

	// Test setting PassCredentialsAll
	g, err = NewHTTPGetter(
		WithBasicAuth("I", "Am"),
		WithPassCredentialsAll(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	hg, ok = g.(*HTTPGetter)
	if !ok {
		t.Fatal("expected NewHTTPGetter to produce an *HTTPGetter")
	}
	if hg.opts.passCredentialsAll != true {
		t.Errorf("Expected NewHTTPGetter to contain %t as PassCredentialsAll, got %t", true, hg.opts.passCredentialsAll)
	}
}

func TestDownload(t *testing.T) {
	expect := "Call me Ishmael"
	expectedUserAgent := "I am Groot"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultUserAgent := "Helm/" + strings.TrimPrefix(version.GetVersion(), "v")
		if r.UserAgent() != defaultUserAgent {
			t.Errorf("Expected '%s', got '%s'", defaultUserAgent, r.UserAgent())
		}
		fmt.Fprint(w, expect)
	}))
	defer srv.Close()

	g, err := All(cli.New()).ByScheme("http")
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.Get(srv.URL, WithURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	if got.String() != expect {
		t.Errorf("Expected %q, got %q", expect, got.String())
	}

	// test with http server
	basicAuthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "username" || password != "password" {
			t.Errorf("Expected request to use basic auth and for username == 'username' and password == 'password', got '%v', '%s', '%s'", ok, username, password)
		}
		if r.UserAgent() != expectedUserAgent {
			t.Errorf("Expected '%s', got '%s'", expectedUserAgent, r.UserAgent())
		}
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
	if err != nil {
		t.Fatal(err)
	}
	got, err = httpgetter.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}

	if got.String() != expect {
		t.Errorf("Expected %q, got %q", expect, got.String())
	}

	// test with Get URL differing from withURL
	crossAuthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok || username == "username" || password == "password" {
			t.Errorf("Expected request to not include but got '%v', '%s', '%s'", ok, username, password)
		}
		fmt.Fprint(w, expect)
	}))

	defer crossAuthSrv.Close()

	u, _ = url.ParseRequestURI(crossAuthSrv.URL)

	// A different host is provided for the WithURL from the one used for Get
	u2, _ := url.ParseRequestURI(crossAuthSrv.URL)
	host := strings.Split(u2.Host, ":")
	host[0] = host[0] + "a"
	u2.Host = strings.Join(host, ":")
	httpgetter, err = NewHTTPGetter(
		WithURL(u2.String()),
		WithBasicAuth("username", "password"),
		WithPassCredentialsAll(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err = httpgetter.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}

	if got.String() != expect {
		t.Errorf("Expected %q, got %q", expect, got.String())
	}

	// test with Get URL differing from withURL and should pass creds
	crossAuthSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "username" || password != "password" {
			t.Errorf("Expected request to use basic auth and for username == 'username' and password == 'password', got '%v', '%s', '%s'", ok, username, password)
		}
		fmt.Fprint(w, expect)
	}))

	defer crossAuthSrv.Close()

	u, _ = url.ParseRequestURI(crossAuthSrv.URL)

	// A different host is provided for the WithURL from the one used for Get
	u2, _ = url.ParseRequestURI(crossAuthSrv.URL)
	host = strings.Split(u2.Host, ":")
	host[0] = host[0] + "a"
	u2.Host = strings.Join(host, ":")
	httpgetter, err = NewHTTPGetter(
		WithURL(u2.String()),
		WithBasicAuth("username", "password"),
		WithPassCredentialsAll(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err = httpgetter.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}

	if got.String() != expect {
		t.Errorf("Expected %q, got %q", expect, got.String())
	}
}

func TestDownloadTLS(t *testing.T) {
	cd := "../../testdata"
	ca, pub, priv := filepath.Join(cd, "rootca.crt"), filepath.Join(cd, "crt.pem"), filepath.Join(cd, "key.pem")

	tlsSrv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tlsConf, err := tlsutil.NewClientTLS(pub, priv, ca)
	if err != nil {
		t.Fatal(errors.Wrap(err, "can't create TLS config for client"))
	}
	tlsConf.BuildNameToCertificate()
	tlsConf.ServerName = "helm.sh"
	tlsSrv.TLS = tlsConf
	tlsSrv.StartTLS()
	defer tlsSrv.Close()

	u, _ := url.ParseRequestURI(tlsSrv.URL)
	g, err := NewHTTPGetter(
		WithURL(u.String()),
		WithTLSClientConfig(pub, priv, ca),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := g.Get(u.String()); err != nil {
		t.Error(err)
	}

	// now test with TLS config being passed along in .Get (see #6635)
	g, err = NewHTTPGetter()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := g.Get(u.String(), WithURL(u.String()), WithTLSClientConfig(pub, priv, ca)); err != nil {
		t.Error(err)
	}

	// test with only the CA file (see also #6635)
	g, err = NewHTTPGetter()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := g.Get(u.String(), WithURL(u.String()), WithTLSClientConfig("", "", ca)); err != nil {
		t.Error(err)
	}
}

func TestDownloadInsecureSkipTLSVerify(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	u, _ := url.ParseRequestURI(ts.URL)

	// Ensure the default behavior did not change
	g, err := NewHTTPGetter(
		WithURL(u.String()),
	)
	if err != nil {
		t.Error(err)
	}

	if _, err := g.Get(u.String()); err == nil {
		t.Errorf("Expected Getter to throw an error, got %s", err)
	}

	// Test certificate check skip
	g, err = NewHTTPGetter(
		WithURL(u.String()),
		WithInsecureSkipVerifyTLS(true),
	)
	if err != nil {
		t.Error(err)
	}
	if _, err = g.Get(u.String()); err != nil {
		t.Error(err)
	}

}

func TestHTTPGetterTarDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, _ := os.Open("testdata/empty-0.0.1.tgz")
		defer f.Close()

		b := make([]byte, 512)
		f.Read(b)
		//Get the file size
		FileStat, _ := f.Stat()
		FileSize := strconv.FormatInt(FileStat.Size(), 10)

		//Simulating improper header values from bitbucket
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", FileSize)

		f.Seek(0, 0)
		io.Copy(w, f)
	}))

	defer srv.Close()

	g, err := NewHTTPGetter(WithURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	data, _ := g.Get(srv.URL)
	mimeType := http.DetectContentType(data.Bytes())

	expectedMimeType := "application/x-gzip"
	if mimeType != expectedMimeType {
		t.Fatalf("Expected response with MIME type %s, but got %s", expectedMimeType, mimeType)
	}
}

func TestHttpClientInsecureSkipVerify(t *testing.T) {
	g := HTTPGetter{}
	g.opts.url = "https://localhost"
	verifyInsecureSkipVerify(t, g, "Blank HTTPGetter", false)

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.caFile = "testdata/ca.crt"
	verifyInsecureSkipVerify(t, g, "HTTPGetter with ca file", false)

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.insecureSkipVerifyTLS = true
	verifyInsecureSkipVerify(t, g, "HTTPGetter with skip cert verification only", true)

	g = HTTPGetter{}
	g.opts.url = "https://localhost"
	g.opts.certFile = "testdata/client.crt"
	g.opts.keyFile = "testdata/client.key"
	g.opts.insecureSkipVerifyTLS = true
	transport := verifyInsecureSkipVerify(t, g, "HTTPGetter with 2 way ssl", true)
	if len(transport.TLSClientConfig.Certificates) <= 0 {
		t.Fatal("transport.TLSClientConfig.Certificates is not present")
	}
	if transport.TLSClientConfig.ServerName == "" {
		t.Fatal("TLSClientConfig.ServerName is blank")
	}
}

func verifyInsecureSkipVerify(t *testing.T, g HTTPGetter, caseName string, expectedValue bool) *http.Transport {
	returnVal, err := g.httpClient()

	if err != nil {
		t.Fatal(err)
	}

	if returnVal == nil {
		t.Fatalf("Expected non nil value for http client")
	}
	transport := (returnVal.Transport).(*http.Transport)
	gotValue := false
	if transport.TLSClientConfig != nil {
		gotValue = transport.TLSClientConfig.InsecureSkipVerify
	}
	if gotValue != expectedValue {
		t.Fatalf("Case Name = %s\nInsecureSkipVerify did not come as expected. Expected = %t; Got = %v",
			caseName, expectedValue, gotValue)
	}
	return transport
}
