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

// NOTE(terryhowe): This fallback feature is only provided in v3 for backward
// compatibility. ORAS v1 had this feature and this code was added when helm
// updated to ORAS v2. This will not be supported in helm v4.

type fallbackTransport struct {
	Base      http.RoundTripper
	forceHTTP atomic.Bool
}

func newTransport(debug bool) *fallbackTransport {
	baseTransport := NewTransport(debug)
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
	// We are falling back to http here for backward compatibility with Helm v3.
	// ORAS v1 provided fallback automatically, but ORAS v2 does not.
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
