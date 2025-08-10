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

package kube

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeRoundTripper struct {
	resp  *http.Response
	err   error
	calls int
}

func (f *fakeRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	f.calls++
	return f.resp, f.err
}

func newRespWithBody(statusCode int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestRetryingRoundTripper_RoundTrip(t *testing.T) {
	marshalErr := func(code int, msg string) string {
		b, _ := json.Marshal(kubernetesError{
			Code:    code,
			Message: msg,
		})
		return string(b)
	}

	tests := []struct {
		name          string
		resp          *http.Response
		err           error
		expectedCalls int
		expectedErr   string
		expectedCode  int
	}{
		{
			name:          "no retry, status < 500 returns response",
			resp:          newRespWithBody(200, "application/json", `{"message":"ok","code":200}`),
			err:           nil,
			expectedCalls: 1,
			expectedCode:  200,
		},
		{
			name:          "error from wrapped RoundTripper propagates",
			resp:          nil,
			err:           errors.New("wrapped error"),
			expectedCalls: 1,
			expectedErr:   "wrapped error",
		},
		{
			name:          "no retry, content-type not application/json",
			resp:          newRespWithBody(500, "text/plain", "server error"),
			err:           nil,
			expectedCalls: 1,
			expectedCode:  500,
		},
		{
			name: "error reading body returns error",
			resp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       &errReader{},
			},
			err:           nil,
			expectedCalls: 1,
			expectedErr:   "read error",
		},
		{
			name:          "error decoding JSON returns error",
			resp:          newRespWithBody(500, "application/json", `invalid-json`),
			err:           nil,
			expectedCalls: 1,
			expectedErr:   "invalid character",
		},
		{
			name:          "retry on etcdserver leader changed message",
			resp:          newRespWithBody(500, "application/json", marshalErr(500, "some error etcdserver: leader changed")),
			err:           nil,
			expectedCalls: 2,
			expectedCode:  500,
		},
		{
			name:          "retry on raft proposal dropped message",
			resp:          newRespWithBody(500, "application/json", marshalErr(500, "rpc error: code = Unknown desc = raft proposal dropped")),
			err:           nil,
			expectedCalls: 2,
			expectedCode:  500,
		},
		{
			name:          "no retry on other error message",
			resp:          newRespWithBody(500, "application/json", marshalErr(500, "other server error")),
			err:           nil,
			expectedCalls: 1,
			expectedCode:  500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRT := &fakeRoundTripper{
				resp: tt.resp,
				err:  tt.err,
			}
			rt := RetryingRoundTripper{
				Wrapped: fakeRT,
			}
			req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
			resp, err := rt.RoundTrip(req)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, resp.StatusCode)
			assert.Equal(t, tt.expectedCalls, fakeRT.calls)
		})
	}
}

type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

func (e *errReader) Close() error {
	return nil
}
