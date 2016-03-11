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
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"
)

var _ Encoder = &AcceptEncoder{}

func TestParseAccept(t *testing.T) {
	e := &AcceptEncoder{
		DefaultEncoding: "application/json",
	}
	tests := map[string]string{
		"":    e.DefaultEncoding,
		"*/*": e.DefaultEncoding,
		// To stay true to spec, this _should_ be an error. But our thought
		// on this case is that we'd rather send a default format.
		"audio/*; q=0.2, audio/basic":                  e.DefaultEncoding,
		"text/html; q=0.8, text/yaml,application/json": "text/yaml",
		"application/x-yaml; foo=bar":                  "application/x-yaml",
		"text/monkey,     TEXT/YAML ; zoom=zoom   ":    "text/yaml",
	}

	for in, expects := range tests {
		mt, enc := e.parseAccept(in)
		if mt != expects {
			t.Errorf("Expected %q, got %q", expects, mt)
			continue
		}
		_, err := enc([]string{"hello", "world"})
		if err != nil {
			t.Fatalf("Failed to marshal: %s", err)
		}
	}
}

func TestTextMarshal(t *testing.T) {
	tests := map[string]interface{}{
		"foo":           "foo",
		"5":             5,
		"stinky cheese": errors.New("stinky cheese"),
	}
	for expect, in := range tests {
		if o, err := textMarshal(in); err != nil || string(o) != expect {
			t.Errorf("Expected %q, got %q", expect, o)
		}
	}

	if _, err := textMarshal(struct{ foo int }{5}); err != ErrUnsupportedKind {
		t.Fatalf("Expected unsupported kind, got %v", err)
	}
}

func TestAcceptEncoder(t *testing.T) {
	c := &Context{
		Encoder: &AcceptEncoder{DefaultEncoding: "application/json"},
	}
	fn := func(w http.ResponseWriter, r *http.Request, c *Context) error {
		c.Encoder.Encode(w, r, c, []string{"hello", "world"})
		return nil
	}
	s := httpHarness(c, "GET /", fn)
	defer s.Close()

	res, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("Unexpected response code %d", res.StatusCode)
	}
	if mt := res.Header.Get("content-type"); mt != "application/json" {
		t.Errorf("Unexpected content type: %q", mt)
	}

	data, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read response body: %s", err)
	}

	out := []string{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %s", err)
	}

	if out[0] != "hello" {
		t.Fatalf("Unexpected JSON data in slot 0: %s", out[0])
	}
}
