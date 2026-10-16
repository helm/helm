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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRoundTripper records the request it receives and returns a canned response.
type captureRoundTripper struct {
	got *http.Request
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, nil
}

func TestSessionRoundTripper(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
	}{
		{name: "sets the session header", sessionID: "test-session-id"},
		{name: "sets the generated session ID", sessionID: NewSessionID()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := &captureRoundTripper{}
			rt := &SessionRoundTripper{Wrapped: capture, SessionID: tt.sessionID}

			req, err := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
			require.NoError(t, err)
			// A pre-existing header with the same name must be replaced, not duplicated.
			req.Header.Set(SessionHeader, "stale")

			_, err = rt.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, capture.got)

			assert.Equal(t, tt.sessionID, capture.got.Header.Get(SessionHeader))
			assert.Len(t, capture.got.Header.Values(SessionHeader), 1)
			// The caller's request must not be mutated.
			assert.Equal(t, "stale", req.Header.Get(SessionHeader))
		})
	}
}

func TestSessionRoundTripper_StableAcrossRequests(t *testing.T) {
	capture := &captureRoundTripper{}
	rt := &SessionRoundTripper{Wrapped: capture, SessionID: NewSessionID()}

	var first string
	for range 3 {
		req, err := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
		require.NoError(t, err)
		_, err = rt.RoundTrip(req)
		require.NoError(t, err)
		got := capture.got.Header.Get(SessionHeader)
		require.NotEmpty(t, got)
		if first == "" {
			first = got
		} else {
			assert.Equal(t, first, got, "session ID must be stable across requests")
		}
	}
}

func TestNewSessionID_UniqueAndNonEmpty(t *testing.T) {
	id1 := NewSessionID()
	id2 := NewSessionID()
	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2, "NewSessionID must return a fresh ID per call")
}
