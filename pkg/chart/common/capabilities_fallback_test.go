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
	"runtime/debug"
	"testing"

	helmversion "helm.sh/helm/v4/internal/version"
)

// TestMakeDefaultCapabilities_FallbackOnBuildInfoFailure simulates the Bazel
// build scenario (issue #31904) where debug.ReadBuildInfo() returns false.
// Verifies that makeDefaultCapabilities falls back to defaults instead of
// returning an error that would trigger the panic in DefaultCapabilities.
func TestMakeDefaultCapabilities_FallbackOnBuildInfoFailure(t *testing.T) {
	// Override isTesting to return false so we bypass the testing early-return
	origIsTesting := isTesting
	isTesting = func() bool { return false }
	t.Cleanup(func() { isTesting = origIsTesting })

	// Override ReadBuildInfo to simulate Bazel (returns false) by which go binary doesn't contain build info.
	origReadBuildInfo := helmversion.ReadBuildInfo
	helmversion.ReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}
	t.Cleanup(func() { helmversion.ReadBuildInfo = origReadBuildInfo })

	caps, err := makeDefaultCapabilities()
	if err != nil {
		t.Fatalf("makeDefaultCapabilities() returned error (would cause panic): %v", err)
	}
	if caps == nil {
		t.Fatal("makeDefaultCapabilities() returned nil capabilities")
	}
	// Should fall back to testing defaults
	if caps.KubeVersion.Major != "1" {
		t.Errorf("Expected fallback Major=1, got %q", caps.KubeVersion.Major)
	}
	if caps.KubeVersion.Minor != "20" {
		t.Errorf("Expected fallback Minor=20, got %q", caps.KubeVersion.Minor)
	}
}

// TestMakeDefaultCapabilities_FallbackOnInvalidVersion simulates a scenario
// where ReadBuildInfo succeeds but the client-go version string is unparseable.
func TestMakeDefaultCapabilities_FallbackOnInvalidVersion(t *testing.T) {
	origIsTesting := isTesting
	isTesting = func() bool { return false }
	t.Cleanup(func() { isTesting = origIsTesting })

	// Return a valid BuildInfo but with an unparseable version
	origReadBuildInfo := helmversion.ReadBuildInfo
	helmversion.ReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Deps: []*debug.Module{
				{Path: "k8s.io/client-go", Version: "not-a-semver"},
			},
		}, true
	}
	t.Cleanup(func() { helmversion.ReadBuildInfo = origReadBuildInfo })

	caps, err := makeDefaultCapabilities()
	if err != nil {
		t.Fatalf("makeDefaultCapabilities() returned error (would cause panic): %v", err)
	}
	if caps.KubeVersion.Major != "1" || caps.KubeVersion.Minor != "20" {
		t.Errorf("Expected fallback v1.20, got v%s.%s", caps.KubeVersion.Major, caps.KubeVersion.Minor)
	}
}
