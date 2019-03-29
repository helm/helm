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

package chartutil

import (
	"encoding/json"
	"testing"
)

func TestVersionSet(t *testing.T) {
	vs := NewVersionSet("v1", "apps/v1")
	if d := len(vs); d != 2 {
		t.Errorf("Expected 2 versions, got %d", d)
	}

	if !vs.Has("apps/v1") {
		t.Error("Expected to find apps/v1")
	}

	if vs.Has("Spanish/inquisition") {
		t.Error("No one expects the Spanish/inquisition")
	}
}

func TestDefaultVersionSet(t *testing.T) {
	if !DefaultVersionSet.Has("v1") {
		t.Error("Expected core v1 version set")
	}
}

func TestCapabilities(t *testing.T) {
	cap := Capabilities{
		APIVersions: DefaultVersionSet,
	}

	if !cap.APIVersions.Has("v1") {
		t.Error("APIVersions should have v1")
	}
}

func TestCapabilitiesJSONMarshal(t *testing.T) {
	vs := NewVersionSet("v1", "apps/v1")
	b, err := json.Marshal(vs)
	if err != nil {
		t.Fatal(err)
	}

	expect := `["apps/v1","v1"]`
	if string(b) != expect {
		t.Fatalf("JSON marshaled semantic version not equal: expected %q, got %q", expect, string(b))
	}
}

func TestCapabilitiesJSONUnmarshal(t *testing.T) {
	in := `["apps/v1","v1"]`

	var vs VersionSet
	if err := json.Unmarshal([]byte(in), &vs); err != nil {
		t.Fatal(err)
	}

	if len(vs) != 2 {
		t.Fatalf("JSON unmarshaled semantic version not equal: expected 2, got %d", len(vs))
	}
}
