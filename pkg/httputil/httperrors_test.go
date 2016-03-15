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

package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotFound(t *testing.T) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		NotFound(w, r)
	}
	testStatusCode(http.HandlerFunc(fn), 404, t)
}

func TestFatal(t *testing.T) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		Fatal(w, r, "fatal %s", "foo")
	}
	testStatusCode(http.HandlerFunc(fn), 500, t)
}

func testStatusCode(fn http.HandlerFunc, expect int, t *testing.T) {
	s := httptest.NewServer(fn)
	defer s.Close()

	res, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != expect {
		t.Errorf("Expected %d, got %d", expect, res.StatusCode)
	}
}
