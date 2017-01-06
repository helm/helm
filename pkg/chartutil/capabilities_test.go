/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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
)

func TestVersionSet(t *testing.T) {
	vs := NewVersionSet("v1", "extensions/v1beta1")
	if d := len(vs); d != 2 {
		t.Errorf("Expected 2 versions, got %d", d)
	}

	if !vs.Has("extensions/v1beta1") {
		t.Error("Expected to find extensions/v1beta1")
	}

	if vs.Has("Spanish/inquisition") {
		t.Error("No one expects the Spanish/inquisition")
	}
}

func TestDefaultVersionSet(t *testing.T) {
	if !DefaultVersionSet.Has("v1") {
		t.Error("Expected core v1 version set")
	}
	if d := len(DefaultVersionSet); d != 1 {
		t.Errorf("Expected only one version, got %d", d)
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
