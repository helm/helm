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
	"testing"
)

func TestVersionSet(t *testing.T) {
	vs := VersionSet{"v1", "apps/v1"}
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

func TestDefaultCapabilities(t *testing.T) {
	kv := DefaultCapabilities.KubeVersion
	if kv.String() != "v1.20.0" {
		t.Errorf("Expected default KubeVersion.String() to be v1.20.0, got %q", kv.String())
	}
	if kv.Version != "v1.20.0" {
		t.Errorf("Expected default KubeVersion.Version to be v1.20.0, got %q", kv.Version)
	}
	if kv.GitVersion() != "v1.20.0" {
		t.Errorf("Expected default KubeVersion.GitVersion() to be v1.20.0, got %q", kv.Version)
	}
	if kv.Major != "1" {
		t.Errorf("Expected default KubeVersion.Major to be 1, got %q", kv.Major)
	}
	if kv.Minor != "20" {
		t.Errorf("Expected default KubeVersion.Minor to be 20, got %q", kv.Minor)
	}
}

func TestDefaultCapabilitiesHelmVersion(t *testing.T) {
	hv := DefaultCapabilities.HelmVersion

	if hv.Version != "v3.10" {
		t.Errorf("Expected default HelmVersion to be v3.10, got %q", hv.Version)
	}
}

func TestParseKubeVersion(t *testing.T) {
	kv, err := ParseKubeVersion("v1.16.0")
	if err != nil {
		t.Errorf("Expected v1.16.0 to parse successfully")
	}
	if kv.Version != "v1.16.0" {
		t.Errorf("Expected parsed KubeVersion.Version to be v1.16.0, got %q", kv.String())
	}
	if kv.Major != "1" {
		t.Errorf("Expected parsed KubeVersion.Major to be 1, got %q", kv.Major)
	}
	if kv.Minor != "16" {
		t.Errorf("Expected parsed KubeVersion.Minor to be 16, got %q", kv.Minor)
	}
}
