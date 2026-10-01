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

package kubeenv

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

// SessionHeader is the HTTP header Helm sends on Kubernetes API requests so
// requests from a single command execution can be correlated for auditing.
const SessionHeader = "helm-session"

// SessionRoundTripper sets the SessionHeader header carrying SessionID on
// every request sent through the wrapped [http.RoundTripper].
type SessionRoundTripper struct {
	Wrapped   http.RoundTripper
	SessionID string
}

// RoundTrip implements [http.RoundTripper].
func (rt *SessionRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request: a RoundTripper must not mutate the request it is given.
	r := req.Clone(req.Context())
	r.Header.Set(SessionHeader, rt.SessionID)
	return rt.Wrapped.RoundTrip(r)
}

// NewSessionID generates a fresh 128-bit session identifier. Callers should
// generate one ID per logical operation: the Helm CLI creates a single
// [cli.EnvSettings] per command invocation, so every Kubernetes client built
// while running one command shares a session, while separate settings (e.g.
// SDK clients performing distinct operations) get distinct sessions.
func NewSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	// crypto/rand effectively never fails; fall back to a timestamp-based value.
	return "fallback-" + strconv.FormatInt(time.Now().UnixNano(), 16)
}
