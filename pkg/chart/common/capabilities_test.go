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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionSet(t *testing.T) {
	vs := VersionSet{"v1", "apps/v1"}
	d := len(vs)
	assert.Equalf(t, 2, d, "Expected 2 versions, got %d", d)

	assert.True(t, vs.Has("apps/v1"), "Expected to find apps/v1")

	assert.False(t, vs.Has("Spanish/inquisition"), "No one expects the Spanish/inquisition")
}

func TestDefaultVersionSet(t *testing.T) {
	assert.True(t, DefaultVersionSet.Has("v1"), "Expected core v1 version set")
}

func TestDefaultCapabilities(t *testing.T) {
	caps := DefaultCapabilities
	kv := caps.KubeVersion
	assert.Equalf(t, "v1.20.0", kv.String(), "Expected default KubeVersion.String() to be v1.20.0, got %q", kv.String())
	assert.Equalf(t, "v1.20.0", kv.Version, "Expected default KubeVersion.Version to be v1.20.0, got %q", kv.Version)
	assert.Equalf(t, "v1.20.0", kv.GitVersion(), "Expected default KubeVersion.GitVersion() to be v1.20.0, got %q", kv.Version)
	assert.Equalf(t, "1", kv.Major, "Expected default KubeVersion.Major to be 1, got %q", kv.Major)
	assert.Equalf(t, "20", kv.Minor, "Expected default KubeVersion.Minor to be 20, got %q", kv.Minor)

	hv := caps.HelmVersion
	assert.Equalf(t, "v4.2", hv.Version, "Expected default HelmVersion to be v4.2, got %q", hv.Version)
}

func TestParseKubeVersion(t *testing.T) {
	kv, err := ParseKubeVersion("v1.16.0")
	require.NoError(t, err, "Expected v1.16.0 to parse successfully")
	assert.Equalf(t, "v1.16.0", kv.Version, "Expected parsed KubeVersion.Version to be v1.16.0, got %q", kv.String())
	assert.Equalf(t, "1", kv.Major, "Expected parsed KubeVersion.Major to be 1, got %q", kv.Major)
	assert.Equalf(t, "16", kv.Minor, "Expected parsed KubeVersion.Minor to be 16, got %q", kv.Minor)
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
			require.NoErrorf(t, err, "ParseKubeVersion()")
			assert.Equalf(t, tt.wantVer, kv.Version, "Version = %q, want %q", kv.Version, tt.wantVer)
			assert.Equalf(t, tt.wantString, kv.String(), "String() = %q, want %q", kv.String(), tt.wantString)
			assert.Equalf(t, tt.wantMajor, kv.Major, "Major = %q, want %q", kv.Major, tt.wantMajor)
			assert.Equalf(t, tt.wantMinor, kv.Minor, "Minor = %q, want %q", kv.Minor, tt.wantMinor)
		})
	}
}
