/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultServerURL(t *testing.T) {
	tt := []struct {
		host string
		url  string
	}{
		{"127.0.0.1", "http://127.0.0.1"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"foo.bar.com", "http://foo.bar.com"},
		{"foo.bar.com/prefix", "http://foo.bar.com/prefix/"},
		{"http://host/prefix", "http://host/prefix/"},
		{"https://host/prefix", "https://host/prefix/"},
		{"http://host", "http://host"},
		{"http://host/other", "http://host/other/"},
	}

	for _, tc := range tt {
		u, err := DefaultServerURL(tc.host)
		if err != nil {
			t.Fatal(err)
		}

		if tc.url != u.String() {
			t.Errorf("%s, expected host %s, got %s", tc.host, tc.url, u.String())
		}
	}
}

func TestURL(t *testing.T) {
	tt := []struct {
		host string
		path string
		url  string
	}{
		{"127.0.0.1", "foo", "http://127.0.0.1/foo"},
		{"127.0.0.1:8080", "foo", "http://127.0.0.1:8080/foo"},
		{"foo.bar.com", "foo", "http://foo.bar.com/foo"},
		{"foo.bar.com/prefix", "foo", "http://foo.bar.com/prefix/foo"},
		{"http://host/prefix", "foo", "http://host/prefix/foo"},
		{"http://host", "foo", "http://host/foo"},
		{"http://host/other", "/foo", "http://host/foo"},
	}

	for _, tc := range tt {
		c := NewClient(tc.host)
		p, err := c.url(tc.path)
		if err != nil {
			t.Fatal(err)
		}

		if tc.url != p {
			t.Errorf("expected %s, got %s", tc.url, p)
		}
	}
}

type fakeClient struct {
	*Client
	server  *httptest.Server
	handler http.HandlerFunc
}

func (c *fakeClient) setup() *fakeClient {
	c.server = httptest.NewServer(c.handler)
	c.Client = NewClient(c.server.URL)
	return c
}

func (c *fakeClient) teardown() {
	c.server.Close()
}

func TestUserAgent(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.UserAgent(), "helm") {
				t.Error("user agent is not set")
			}
		}),
	}
	fc.setup().ListDeployments()
}
