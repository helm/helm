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

package common

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
	caps := DefaultCapabilities
	kv := caps.KubeVersion
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

	hv := caps.HelmVersion
	if hv.Version != "v4.1" {
		t.Errorf("Expected default HelmVersion to be v4.1, got %q", hv.Version)
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

func TestParseKubeVersionWithVendorSuffixes(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantVer    string
		wantString string
		wantMajor  string
		wantMinor  string
	}{
		{"GKE vendor suffix", "v1.33.4-gke.1245000", "v1.33.4-gke.1245000", "v1.33.4", "1", "33"},
		{"GKE without v", "1.30.2-gke.1587003", "v1.30.2-gke.1587003", "v1.30.2", "1", "30"},
		{"EKS trailing +", "v1.28+", "v1.28+", "v1.28", "1", "28"},
		{"EKS + without v", "1.28+", "v1.28+", "v1.28", "1", "28"},
		{"Standard version", "v1.31.0", "v1.31.0", "v1.31.0", "1", "31"},
		{"Standard without v", "1.29.0", "v1.29.0", "v1.29.0", "1", "29"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv, err := ParseKubeVersion(tt.input)
			if err != nil {
				t.Fatalf("ParseKubeVersion() error = %v", err)
			}
			if kv.Version != tt.wantVer {
				t.Errorf("Version = %q, want %q", kv.Version, tt.wantVer)
			}
			if kv.String() != tt.wantString {
				t.Errorf("String() = %q, want %q", kv.String(), tt.wantString)
			}
			if kv.Major != tt.wantMajor {
				t.Errorf("Major = %q, want %q", kv.Major, tt.wantMajor)
			}
			if kv.Minor != tt.wantMinor {
				t.Errorf("Minor = %q, want %q", kv.Minor, tt.wantMinor)
			}
		})
	}
}
