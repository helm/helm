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

package registry

import (
	"testing"
)

func TestTypeConversion(t *testing.T) {
	// TODO: Are there some real-world examples we want to validate here?
	tests := map[string]Type{
		"foo":                   NewTypeOrDie("", "foo", ""),
		"foo:v1":                NewTypeOrDie("", "foo", "v1"),
		"github.com/foo":        NewTypeOrDie("github.com", "foo", ""),
		"github.com/foo:v1.2.3": NewTypeOrDie("github.com", "foo", "v1.2.3"),
	}

	for in, expect := range tests {
		out, err := ParseType(in)
		if err != nil {
			t.Errorf("Error parsing type string %s: %s", in, err)
		}

		if out.Name != expect.Name {
			t.Errorf("Expected name to be %q, got %q", expect.Name, out.Name)
		}

		if out.GetVersion() != expect.GetVersion() {
			t.Errorf("Expected version to be %q, got %q", expect.GetVersion(), out.GetVersion())
		}

		if out.Collection != expect.Collection {
			t.Errorf("Expected collection to be %q, got %q", expect.Collection, out.Collection)
		}

		svalue := out.String()
		if svalue != in {
			t.Errorf("Expected string value to be %q, got %q", in, svalue)
		}
	}
}
