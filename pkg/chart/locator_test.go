/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package chart

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := map[string]Locator{
		"helm:host/bucket/name#1.2.3":        {Scheme: "helm", Host: "host", Bucket: "bucket", Name: "name", Version: "1.2.3"},
		"https://host/bucket/name-1.2.3.tgz": {Scheme: "https", Host: "host", Bucket: "bucket", Name: "name", Version: "1.2.3"},
		"http://host/bucket/name-1.2.3.tgz":  {Scheme: "http", Host: "host", Bucket: "bucket", Name: "name", Version: "1.2.3"},
	}

	for start, expect := range tests {
		u, err := Parse(start)
		if err != nil {
			t.Errorf("Failed parsing %s: %s", start, err)
		}

		if expect.Scheme != u.Scheme {
			t.Errorf("Unexpected scheme: %s", u.Scheme)
		}

		if expect.Host != u.Host {
			t.Errorf("Unexpected host: %q", u.Host)
		}

		if expect.Bucket != u.Bucket {
			t.Errorf("Unexpected bucket: %q", u.Bucket)
		}

		if expect.Name != u.Name {
			t.Errorf("Unexpected name: %q", u.Name)
		}

		if expect.Version != u.Version {
			t.Errorf("Unexpected version: %q", u.Version)
		}

		if expect.LocalRef != u.LocalRef {
			t.Errorf("Unexpected local dir: %q", u.LocalRef)
		}

	}
}

func TestShort(t *testing.T) {
	tests := map[string]string{
		"https://example.com/foo/bar-1.2.3.tgz": "helm:example.com/foo/bar#1.2.3",
		"http://example.com/foo/bar-1.2.3.tgz":  "helm:example.com/foo/bar#1.2.3",
		"helm:example.com/foo/bar#1.2.3":        "helm:example.com/foo/bar#1.2.3",
		"helm:example.com/foo/bar#>1.2.3":       "helm:example.com/foo/bar#%3E1.2.3",
	}

	for start, expect := range tests {
		u, err := Parse(start)
		if err != nil {
			t.Errorf("Failed to parse: %s", err)
			continue
		}
		short, err := u.Short()
		if err != nil {
			t.Errorf("Failed to generate short: %s", err)
			continue
		}

		if short != expect {
			t.Errorf("Expected %q, got %q", expect, short)
		}
	}

	fails := []string{"./this/is/local", "file:///this/is/local"}
	for _, f := range fails {
		u, err := Parse(f)
		if err != nil {
			t.Errorf("Failed to parse: %s", err)
			continue
		}

		if _, err := u.Short(); err == nil {
			t.Errorf("%q should have caused an error for Short()", f)
		}
	}
}

func TestLong(t *testing.T) {
	tests := map[string]string{
		"https://example.com/foo/bar-1.2.3.tgz": "https://example.com/foo/bar-1.2.3.tgz",
		"http://example.com/foo/bar-1.2.3.tgz":  "https://example.com/foo/bar-1.2.3.tgz",
		"helm:example.com/foo/bar#1.2.3":        "https://example.com/foo/bar-1.2.3.tgz",
		"helm:example.com/foo/bar#>1.2.3":       "https://example.com/foo/bar-%3E1.2.3.tgz",
	}

	for start, expect := range tests {
		t.Logf("Parsing %s", start)
		u, err := Parse(start)
		if err != nil {
			t.Errorf("Failed to parse: %s", err)
			continue
		}
		long, err := u.Long(true)
		if err != nil {
			t.Errorf("Failed to generate long: %s", err)
			continue
		}

		if long != expect {
			t.Errorf("Expected %q, got %q", expect, long)
		}
	}

	fails := []string{"./this/is/local", "file:///this/is/local"}
	for _, f := range fails {
		u, err := Parse(f)
		if err != nil {
			t.Errorf("Failed to parse: %s", err)
			continue
		}

		if _, err := u.Long(false); err == nil {
			t.Errorf("%q should have caused an error for Long()", f)
		}
	}
}

func TestLocal(t *testing.T) {
	tests := map[string]string{
		"file:///foo/bar-1.2.3.tgz":  "/foo/bar-1.2.3.tgz",
		"file:///foo/bar":            "/foo/bar",
		"./foo/bar":                  "./foo/bar",
		"/foo/bar":                   "/foo/bar",
		"file://localhost/etc/fstab": "/etc/fstab",
		// https://blogs.msdn.microsoft.com/ie/2006/12/06/file-uris-in-windows/
		"file:///C:/WINDOWS/clock.avi": "/C:/WINDOWS/clock.avi",
	}

	for start, expect := range tests {
		u, err := Parse(start)
		if err != nil {
			t.Errorf("Failed parse: %s", err)
			continue
		}

		fin, err := u.Local()
		if err != nil {
			t.Errorf("Failed Local(): %s", err)
			continue
		}

		if fin != expect {
			t.Errorf("Expected %q, got %q", expect, fin)
		}
	}

}

func TestParseTarName(t *testing.T) {
	tests := []struct{ start, name, version string }{
		{"butcher-1.2.3", "butcher", "1.2.3"},
		{"butcher-1.2.3.tgz", "butcher", "1.2.3"},
		{"butcher-1.2.3-beta1+1234", "butcher", "1.2.3-beta1+1234"},
		{"butcher-1.2.3-beta1+1234.tgz", "butcher", "1.2.3-beta1+1234"},
		{"foo/butcher-1.2.3.tgz", "foo/butcher", "1.2.3"},
	}

	for _, tt := range tests {
		n, v, e := parseTarName(tt.start)
		if e != nil {
			t.Errorf("Error parsing %s: %s", tt.start, e)
			continue
		}
		if n != tt.name {
			t.Errorf("Expected name %q, got %q", tt.name, n)
		}

		if v != tt.version {
			t.Errorf("Expected version %q, got %q", tt.version, v)
		}
	}
}
