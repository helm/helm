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
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"oras.land/oras-go/v2/registry/remote/retry"
)

var (
	// requestCount records the number of logged request-response pairs and will
	// be used as the unique id for the next pair.
	requestCount uint64

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
	logger *slog.Logger
}

// NewTransport creates and returns a new instance of LoggingTransport
func NewTransport(debug bool) *retry.Transport {
	type cloner[T any] interface {
		Clone() T
	}

	// try to copy (clone) the http.DefaultTransport so any mutations we
	// perform on it (e.g. TLS config) are not reflected globally
	// follow https://github.com/golang/go/issues/39299 for a more elegant
	// solution in the future
	transport := http.DefaultTransport
	if t, ok := transport.(cloner[*http.Transport]); ok {
		transport = t.Clone()
	} else if t, ok := transport.(cloner[http.RoundTripper]); ok {
		// this branch will not be used with go 1.20, it was added
		// optimistically to try to clone if the http.DefaultTransport
		// implementation changes, still the Clone method in that case
		// might not return http.RoundTripper...
		transport = t.Clone()
	}
	if debug {
		replace := func(groups []string, a slog.Attr) slog.Attr {
			// Remove time.
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			ReplaceAttr: replace,
			Level:       slog.LevelDebug}))
		transport = &LoggingTransport{RoundTripper: transport, logger: logger}
	}

	return retry.NewTransport(transport)
}

// RoundTrip calls base round trip while keeping track of the current request.
func (t *LoggingTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	id := atomic.AddUint64(&requestCount, 1) - 1

	t.logger.Debug("Request", "id", id, "url", req.URL, "method", req.Method, "header", logHeader(req.Header))
	resp, err = t.RoundTripper.RoundTrip(req)
	if err != nil {
		t.logger.Debug("Response", "id", id, "error", err)
	} else if resp != nil {
		t.logger.Debug("Response", "id", id, "status", resp.Status, "header", logHeader(resp.Header), "body", logResponseBody(resp))
	} else {
		t.logger.Debug("Response", "id", id, "response", "nil")
	}

	return resp, err
}

// logHeader prints out the provided header keys and values, with auth header scrubbed.
func logHeader(header http.Header) string {
	if len(header) > 0 {
		headers := []string{}
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
	if _, err := io.CopyN(buf, body, payloadSizeLimit+1); err != nil && err != io.EOF {
		return fmt.Sprintf("   Error reading response body: %v", err)
	}

	readBody := buf.String()
	if len(readBody) == 0 {
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
