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

package cmd

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/version"
)

func TestVersion(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "default",
		cmd:    "version",
		golden: "output/version.txt",
	}, {
		name:   "short",
		cmd:    "version --short",
		golden: "output/version-short.txt",
	}, {
		name:   "template",
		cmd:    "version --template='Version: {{.Version}}'",
		golden: "output/version-template.txt",
	}}
	runTestCmd(t, tests)
}

func TestVersionFileCompletion(t *testing.T) {
	checkFileCompletion(t, "version", false)
}

// TestVersionReportsKubeClient ensures the helm binary keeps client-go linked so
// `helm version` still reports KubeClientVersion. clientgo.go no longer forces
// the dependency, so guard the CLI's own linkage here (this test binary mirrors
// it) to catch a future change that unlinks client-go from the command surface.
func TestVersionReportsKubeClient(t *testing.T) {
	if _, ok := debug.ReadBuildInfo(); !ok {
		t.Skip("build info unavailable (Go < 1.27)")
	}
	v, err := version.K8sIOClientGoModVersion()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(v, "v"), "expected client-go version, got %q", v)
}
