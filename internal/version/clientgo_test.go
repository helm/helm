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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestK8sClientGoModVersion(t *testing.T) {
	// Generated test binaries omit dependency modules from embedded build info,
	// even when the package under test imports them. The built-binary test below
	// covers the normal executable path where dependency modules are present.
	_, err := K8sIOClientGoModVersion()
	require.ErrorContains(t, err, "k8s.io/client-go not found in build info")
}

func TestK8sClientGoModVersionFromBuiltBinary(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-f={{.Version}}", "k8s.io/client-go")
	expectedVersion, err := cmd.CombinedOutput()
	require.NoError(t, err, string(expectedVersion))

	binary := filepath.Join(t.TempDir(), "clientgo-version")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	cmd = exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./testdata/clientgo-version")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	cmd = exec.CommandContext(t.Context(), binary)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Equal(t, strings.TrimSpace(string(expectedVersion)), strings.TrimSpace(string(output)))
}

func TestK8sClientGoDependencyIsLightweight(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", ".")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	dependencies := strings.Fields(string(output))
	require.Contains(t, dependencies, "k8s.io/client-go/pkg/version")
	for _, dependency := range dependencies {
		require.NotEqual(t, "k8s.io/client-go/kubernetes", dependency)
		require.NotContains(t, dependency, "k8s.io/client-go/kubernetes/")
	}
}
