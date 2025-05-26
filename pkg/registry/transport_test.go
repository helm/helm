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
	"io"
	"net/http"
	"testing"
)

var errMockRead = errors.New("mock read error")

type errorReader struct{}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, errMockRead
}

func Test_isPrintableContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{
			name:        "Empty content type",
			contentType: "",
			want:        false,
		},
		{
			name:        "General JSON type",
			contentType: "application/json",
			want:        true,
		},
		{
			name:        "General JSON type with charset",
			contentType: "application/json; charset=utf-8",
			want:        true,
		},
		{
			name:        "Random type with application/json prefix",
			contentType: "application/jsonwhatever",
			want:        false,
		},
		{
			name:        "Manifest type in JSON",
			contentType: "application/vnd.oci.image.manifest.v1+json",
			want:        true,
		},
		{
			name:        "Manifest type in JSON with charset",
			contentType: "application/vnd.oci.image.manifest.v1+json; charset=utf-8",
			want:        true,
		},
		{
			name:        "Random content type in JSON",
			contentType: "application/whatever+json",
			want:        true,
		},
		{
			name:        "Plain text type",
			contentType: "text/plain",
			want:        true,
		},
		{
			name:        "Plain text type with charset",
			contentType: "text/plain; charset=utf-8",
			want:        true,
		},
		{
			name:        "Random type with text/plain prefix",
			contentType: "text/plainnnnn",
			want:        false,
		},
		{
			name:        "HTML type",
			contentType: "text/html",
			want:        true,
		},
		{
			name:        "Plain text type with charset",
			contentType: "text/html; charset=utf-8",
			want:        true,
		},
		{
			name:        "Random type with text/html prefix",
			contentType: "text/htmlllll",
			want:        false,
		},
		{
			name:        "Binary type",
			contentType: "application/octet-stream",
			want:        false,
		},
		{
			name:        "Unknown type",
			contentType: "unknown/unknown",
			want:        false,
		},
		{
			name:        "Invalid type",
			contentType: "text/",
			want:        false,
		},
		{
			name:        "Random string",
			contentType: "random123!@#",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrintableContentType(tt.contentType); got != tt.want {
				t.Errorf("isPrintableContentType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_logResponseBody(t *testing.T) {
	tests := []struct {
		name     string
		resp     *http.Response
		want     string
		wantData []byte
	}{
		{
			name: "Nil body",
			resp: &http.Response{
				Body:   nil,
				Header: http.Header{"Content-Type": []string{"application/json"}},
			},
			want: "   No response body to print",
		},
		{
			name:     "No body",
			wantData: nil,
			resp: &http.Response{
				Body:          http.NoBody,
				ContentLength: 100, // in case of HEAD response, the content length is set but the body is empty
				Header:        http.Header{"Content-Type": []string{"application/json"}},
			},
			want: "   No response body to print",
		},
		{
			name:     "Empty body",
			wantData: []byte(""),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte(""))),
				ContentLength: 0,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: "   Response body is empty",
		},
		{
			name:     "Unknown content length",
			wantData: []byte("whatever"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("whatever"))),
				ContentLength: -1,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: "whatever",
		},
		{
			name:     "Missing content type header",
			wantData: []byte("whatever"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("whatever"))),
				ContentLength: 8,
			},
			want: "   Response body without a content type is not printed",
		},
		{
			name:     "Empty content type header",
			wantData: []byte("whatever"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("whatever"))),
				ContentLength: 8,
				Header:        http.Header{"Content-Type": []string{""}},
			},
			want: "   Response body without a content type is not printed",
		},
		{
			name:     "Non-printable content type",
			wantData: []byte("binary data"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("binary data"))),
				ContentLength: 11,
				Header:        http.Header{"Content-Type": []string{"application/octet-stream"}},
			},
			want: "   Response body of content type \"application/octet-stream\" is not printed",
		},
		{
			name:     "Body at the limit",
			wantData: bytes.Repeat([]byte("a"), int(payloadSizeLimit)),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), int(payloadSizeLimit)))),
				ContentLength: payloadSizeLimit,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: string(bytes.Repeat([]byte("a"), int(payloadSizeLimit))),
		},
		{
			name:     "Body larger than limit",
			wantData: bytes.Repeat([]byte("a"), int(payloadSizeLimit+1)),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), int(payloadSizeLimit+1)))), // 1 byte larger than limit
				ContentLength: payloadSizeLimit + 1,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: string(bytes.Repeat([]byte("a"), int(payloadSizeLimit))) + "\n...(truncated)",
		},
		{
			name:     "Printable content type within limit",
			wantData: []byte("data"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("data"))),
				ContentLength: 4,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: "data",
		},
		{
			name:     "Actual body size is larger than content length",
			wantData: []byte("data"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("data"))),
				ContentLength: 3, // mismatched content length
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: "data",
		},
		{
			name:     "Actual body size is larger than content length and exceeds limit",
			wantData: bytes.Repeat([]byte("a"), int(payloadSizeLimit+1)),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), int(payloadSizeLimit+1)))), // 1 byte larger than limit
				ContentLength: 1,                                                                                 // mismatched content length
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: string(bytes.Repeat([]byte("a"), int(payloadSizeLimit))) + "\n...(truncated)",
		},
		{
			name:     "Actual body size is smaller than content length",
			wantData: []byte("data"),
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte("data"))),
				ContentLength: 5, // mismatched content length
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: "data",
		},
		{
			name: "Body contains token",
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte(`{"token":"12345"}`))),
				ContentLength: 17,
				Header:        http.Header{"Content-Type": []string{"application/json"}},
			},
			wantData: []byte(`{"token":"12345"}`),
			want:     "   Response body redacted due to potential credentials",
		},
		{
			name: "Body contains access_token",
			resp: &http.Response{
				Body:          io.NopCloser(bytes.NewReader([]byte(`{"access_token":"12345"}`))),
				ContentLength: 17,
				Header:        http.Header{"Content-Type": []string{"application/json"}},
			},
			wantData: []byte(`{"access_token":"12345"}`),
			want:     "   Response body redacted due to potential credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logResponseBody(tt.resp); got != tt.want {
				t.Errorf("logResponseBody() = %v, want %v", got, tt.want)
			}
			// validate the response body
			if tt.resp.Body != nil {
				readBytes, err := io.ReadAll(tt.resp.Body)
				if err != nil {
					t.Errorf("failed to read body after logResponseBody(), err= %v", err)
				}
				if !bytes.Equal(readBytes, tt.wantData) {
					t.Errorf("resp.Body after logResponseBody() = %v, want %v", readBytes, tt.wantData)
				}
				if closeErr := tt.resp.Body.Close(); closeErr != nil {
					t.Errorf("failed to close body after logResponseBody(), err= %v", closeErr)
				}
			}
		})
	}
}

func Test_logResponseBody_error(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		want string
	}{
		{
			name: "Error reading body",
			resp: &http.Response{
				Body:          io.NopCloser(&errorReader{}),
				ContentLength: 10,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
			},
			want: "   Error reading response body: mock read error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logResponseBody(tt.resp); got != tt.want {
				t.Errorf("logResponseBody() = %v, want %v", got, tt.want)
			}
			if closeErr := tt.resp.Body.Close(); closeErr != nil {
				t.Errorf("failed to close body after logResponseBody(), err= %v", closeErr)
			}
		})
	}
}

func Test_containsCredentials(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "Contains token keyword",
			body: `{"token": "12345"}`,
			want: true,
		},
		{
			name: "Contains quoted token keyword",
			body: `whatever "token" blah`,
			want: true,
		},
		{
			name: "Contains unquoted token keyword",
			body: `whatever token blah`,
			want: false,
		},
		{
			name: "Contains access_token keyword",
			body: `{"access_token": "12345"}`,
			want: true,
		},
		{
			name: "Contains quoted access_token keyword",
			body: `whatever "access_token" blah`,
			want: true,
		},
		{
			name: "Contains unquoted access_token keyword",
			body: `whatever access_token blah`,
			want: false,
		},
		{
			name: "Does not contain credentials",
			body: `{"key": "value"}`,
			want: false,
		},
		{
			name: "Empty body",
			body: ``,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsCredentials(tt.body); got != tt.want {
				t.Errorf("containsCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}
