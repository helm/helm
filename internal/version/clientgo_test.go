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

package version

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestK8sClientGoModVersion(t *testing.T) {
	// Unfortunately, test builds don't include debug info / module info
	// So we expect "K8sIOClientGoModVersion" to return error
	_, err := K8sIOClientGoModVersion()
	require.ErrorContains(t, err, "k8s.io/client-go not found in build info")
}

func TestK8sClientGoModVersion_ReadBuildInfoFalse(t *testing.T) {
	// Simulate Bazel builds where ReadBuildInfo returns false
	orig := ReadBuildInfo
	ReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}
	t.Cleanup(func() { ReadBuildInfo = orig })

	_, err := K8sIOClientGoModVersion()
	require.ErrorContains(t, err, "failed to read build info")
}

func TestK8sClientGoModVersion_NilBuildInfo(t *testing.T) {
	// Simulate edge case where ok=true but info is nil
	orig := ReadBuildInfo
	ReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, true
	}
	t.Cleanup(func() { ReadBuildInfo = orig })

	_, err := K8sIOClientGoModVersion()
	require.ErrorContains(t, err, "failed to read build info")
}

func TestK8sClientGoModVersion_NilDeps(t *testing.T) {
	// Simulate Bazel builds with empty build info (no deps)
	orig := ReadBuildInfo
	ReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{}, true
	}
	t.Cleanup(func() { ReadBuildInfo = orig })

	_, err := K8sIOClientGoModVersion()
	require.ErrorContains(t, err, "k8s.io/client-go not found in build info")
}

func TestK8sClientGoModVersion_WithClientGo(t *testing.T) {
	orig := ReadBuildInfo
	ReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Deps: []*debug.Module{
				{Path: "k8s.io/client-go", Version: "v0.31.0"},
			},
		}, true
	}
	t.Cleanup(func() { ReadBuildInfo = orig })

	v, err := K8sIOClientGoModVersion()
	require.NoError(t, err)
	require.Equal(t, "v0.31.0", v)
}
