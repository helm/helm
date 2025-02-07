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
	"crypto/tls"
	"net/http"
	"sync/atomic"
)

type fallbackTransport struct {
	Base      http.RoundTripper
	forceHTTP atomic.Bool
}

func newTransport() *fallbackTransport {
	type cloner[T any] interface {
		Clone() T
	}
	// try to copy (clone) the http.DefaultTransport so any mutations we
	// perform on it (e.g. TLS config) are not reflected globally
	// follow https://github.com/golang/go/issues/39299 for a more elegant
	// solution in the future
	baseTransport := http.DefaultTransport
	if t, ok := baseTransport.(cloner[*http.Transport]); ok {
		baseTransport = t.Clone()
	} else if t, ok := baseTransport.(cloner[http.RoundTripper]); ok {
		// this branch will not be used with go 1.20, it was added
		// optimistically to try to clone if the http.DefaultTransport
		// implementation changes, still the Clone method in that case
		// might not return http.RoundTripper...
		baseTransport = t.Clone()
	}

	return &fallbackTransport{
		Base: baseTransport,
	}
}

// RoundTrip wraps base round trip with conditional insecure retry.
func (t *fallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if ok := t.forceHTTP.Load(); ok {
		req.URL.Scheme = "http"
		return t.Base.RoundTrip(req)
	}
	resp, err := t.Base.RoundTrip(req)
	if err != nil && req.URL.Scheme == "https" {
		if tlsErr, ok := err.(tls.RecordHeaderError); ok {
			if string(tlsErr.RecordHeader[:]) == "HTTP/" {
				t.forceHTTP.Store(true)
				req.URL.Scheme = "http"
				return t.Base.RoundTrip(req)
			}
		}
	}
	return resp, err
}
