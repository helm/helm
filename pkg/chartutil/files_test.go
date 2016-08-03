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

package chartutil

import (
	"testing"

	"github.com/golang/protobuf/ptypes/any"
)

func TestNewFiles(t *testing.T) {

	cases := []struct {
		path, data string
	}{
		{"ship/captain.txt", "The Captain"},
		{"ship/stowaway.txt", "Legatt"},
		{"story/name.txt", "The Secret Sharer"},
		{"story/author.txt", "Joseph Conrad"},
	}

	a := []*any.Any{}
	for _, c := range cases {
		a = append(a, &any.Any{TypeUrl: c.path, Value: []byte(c.data)})
	}

	files := NewFiles(a)
	if len(files) != len(cases) {
		t.Errorf("Expected len() = %d, got %d", len(cases), len(files))
	}

	for i, f := range cases {
		if got := string(files.GetBytes(f.path)); got != f.data {
			t.Errorf("%d: expected %q, got %q", i, f.data, got)
		}
		if got := files.Get(f.path); got != f.data {
			t.Errorf("%d: expected %q, got %q", i, f.data, got)
		}
	}
}
