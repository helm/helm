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

package dm

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugTransport(t *testing.T) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"awesome"}`))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	var output bytes.Buffer

	client := &http.Client{
		Transport: debugTransport{
			Writer: &output,
		},
	}

	_, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err.Error())
	}

	expected := []string{
		"GET / HTTP/1.1",
		"Accept-Encoding: gzip",
		"HTTP/1.1 200 OK",
		"Content-Length: 20",
		"Content-Type: application/json",
		`{"status":"awesome"}`,
	}
	actual := output.String()

	for _, match := range expected {
		if !strings.Contains(actual, match) {
			t.Errorf("Expected %s to contain %s", actual, match)
		}
	}
}
