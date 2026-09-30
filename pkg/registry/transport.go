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

package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"oras.land/oras-go/v2/registry/remote/retry"
)

var (
	// requestCount records the number of logged request-response pairs and will
	// be used as the unique id for the next pair.
	requestCount atomic.Uint64

	// toScrub is a set of headers that should be scrubbed from the log.
	toScrub = []string{
		"Authorization",
		"Set-Cookie",
	}
)

// payloadSizeLimit limits the maximum size of the response body to be printed.
const payloadSizeLimit int64 = 16 * 1024 // 16 KiB

// LoggingTransport is an http.RoundTripper that keeps track of the in-flight
// request and add hooks to report HTTP tracing events.
type LoggingTransport struct {
	http.RoundTripper
}

// NewTransport creates and returns a new instance of LoggingTransport
func NewTransport(debug bool) *retry.Transport {
	transport := defaultTransport()
	if debug {
		transport = &LoggingTransport{RoundTripper: transport}
	}

	return retry.NewTransport(transport)
}

// defaultTransport returns the transport a registry client starts from. It is
// always a value Helm owns, because TLS settings are applied to it afterwards
// (see ensureTLSConfig) and must not be applied to the shared
// http.DefaultTransport.
func defaultTransport() http.RoundTripper {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		return t.Clone()
	}

	// http.DefaultTransport is a variable of interface type, so an imported package
	// can replace it with a RoundTripper that cannot be cloned and cannot carry a
	// TLS configuration. Use the standard library defaults rather than handing out
	// that replacement, which would make every TLS option fail to apply.
	slog.Warn("http.DefaultTransport is not an *http.Transport, using the default transport settings instead",
		"transport", fmt.Sprintf("%T", http.DefaultTransport))
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// RoundTrip calls base round trip while keeping track of the current request.
func (t *LoggingTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	id := requestCount.Add(1) - 1

	slog.Debug(req.Method, "id", id, "url", req.URL, "header", logHeader(req.Header))
	resp, err = t.RoundTripper.RoundTrip(req)
	switch {
	case err != nil:
		slog.Debug("Response"[:len(req.Method)], "id", id, "error", err)
	case resp != nil:
		slog.Debug("Response"[:len(req.Method)], "id", id, "status", resp.Status, "header", logHeader(resp.Header), "body", logResponseBody(resp))
	default:
		slog.Debug("Response"[:len(req.Method)], "id", id, "response", "nil")
	}

	return resp, err
}

// logHeader prints out the provided header keys and values, with auth header scrubbed.
func logHeader(header http.Header) string {
	if len(header) > 0 {
		var headers []string
		for k, v := range header {
			for _, h := range toScrub {
				if strings.EqualFold(k, h) {
					v = []string{"*****"}
				}
			}
			headers = append(headers, fmt.Sprintf("   %q: %q", k, strings.Join(v, ", ")))
		}
		return strings.Join(headers, "\n")
	}
	return "   Empty header"
}

// logResponseBody prints out the response body if it is printable and within size limit.
func logResponseBody(resp *http.Response) string {
	if resp.Body == nil || resp.Body == http.NoBody {
		return "   No response body to print"
	}

	// non-applicable body is not printed and remains untouched for subsequent processing
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return "   Response body without a content type is not printed"
	}
	if !isPrintableContentType(contentType) {
		return fmt.Sprintf("   Response body of content type %q is not printed", contentType)
	}

	buf := bytes.NewBuffer(nil)
	body := resp.Body
	// restore the body by concatenating the read body with the remaining body
	resp.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(buf, body),
		Closer: body,
	}
	// read the body up to limit+1 to check if the body exceeds the limit
	if _, err := io.CopyN(buf, body, payloadSizeLimit+1); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Sprintf("   Error reading response body: %v", err)
	}

	readBody := buf.String()
	if readBody == "" {
		return "   Response body is empty"
	}
	if containsCredentials(readBody) {
		return "   Response body redacted due to potential credentials"
	}
	if len(readBody) > int(payloadSizeLimit) {
		return readBody[:payloadSizeLimit] + "\n...(truncated)"
	}
	return readBody
}

// isPrintableContentType returns true if the contentType is printable.
func isPrintableContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	switch mediaType {
	case "application/json", // JSON types
		"text/plain", "text/html": // text types
		return true
	}
	return strings.HasSuffix(mediaType, "+json")
}

// containsCredentials returns true if the body contains potential credentials.
func containsCredentials(body string) bool {
	return strings.Contains(body, `"token"`) || strings.Contains(body, `"access_token"`)
}
