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
	"fmt"
	"runtime"
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
	expectedKubeMajorVersion := "1"
	expectedKubeMinorVersion := "18"
	expectedKubeVersion := fmt.Sprintf("v%s.%s.0", expectedKubeMajorVersion, expectedKubeMinorVersion)
	kv := DefaultCapabilities.KubeVersion
	if kv.String() != expectedKubeVersion {
		t.Errorf("Expected default KubeVersion.String() to be v1.18.0, got %q", kv.String())
	}
	if kv.Version != expectedKubeVersion {
		t.Errorf("Expected default KubeVersion.Version to be v1.18.0, got %q", kv.Version)
	}
	if kv.GitVersion() != expectedKubeVersion {
		t.Errorf("Expected default KubeVersion.GitVersion() to be v1.18.0, got %q", kv.Version)
	}
	if kv.Major != expectedKubeMajorVersion {
		t.Errorf("Expected default KubeVersion.Major to be %s, got %q", expectedKubeMajorVersion, kv.Major)

	}
	if kv.Minor != expectedKubeMinorVersion {
		t.Errorf("Expected default KubeVersion.Minor to be %s, got %q", expectedKubeMinorVersion, kv.Minor)
	}
	if kv.GoVersion != runtime.Version() {
		t.Errorf("Expected default KubeVersion.GoVersion to be %q, got %q", runtime.Version(), kv.GoVersion)
	}
	if kv.Compiler != runtime.Compiler {
		t.Errorf("Expected default KubeVersion.Compiler to be %q, got %q", runtime.Compiler, kv.Compiler)
	}
	expectedPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	if kv.Platform != expectedPlatform {
		t.Errorf("Expected default KubeVersion.Platform to be %q, got %q", expectedPlatform, kv.Platform)
	}
}

func TestDefaultCapabilitiesHelmVersion(t *testing.T) {
	hv := DefaultCapabilities.HelmVersion

	if hv.Version != "v3.2" {
		t.Errorf("Expected default HelmVerison to be v3.2, got %q", hv.Version)
	}
}
