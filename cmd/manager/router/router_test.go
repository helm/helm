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

package router

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	c := &Context{}

	h := NewHandler(c)
	h.Add("GET /", func(w http.ResponseWriter, r *http.Request, c *Context) error {
		fmt.Fprintln(w, "hello")
		return nil
	})
	h.Add("POST /", func(w http.ResponseWriter, r *http.Request, c *Context) error {
		fmt.Fprintln(w, "goodbye")
		return nil
	})

	s := httptest.NewServer(h)
	defer s.Close()

	res, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	if "hello\n" != string(data) {
		t.Errorf("Expected 'hello', got %q", data)
	}
}
