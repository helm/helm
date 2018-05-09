/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

// Package version represents the current version of the project.
package version // import "k8s.io/helm/pkg/version"

import "testing"

func TestBuildInfo(t *testing.T) {
	tests := []struct {
		version       string
		buildMetadata string
		gitCommit     string
		gitTreeState  string
		expected      BuildInfo
	}{
		{"", "", "", "", BuildInfo{Version: "", GitCommit: "", GitTreeState: ""}},
		{"v1.0.0", "", "", "", BuildInfo{Version: "v1.0.0", GitCommit: "", GitTreeState: ""}},
		{"v1.0.0", "79d5c5f7", "", "", BuildInfo{Version: "v1.0.0+79d5c5f7", GitCommit: "", GitTreeState: ""}},
		{"v1.0.0", "79d5c5f7", "0d399baec2acda578a217d1aec8d7d707c71e44d", "", BuildInfo{Version: "v1.0.0+79d5c5f7", GitCommit: "0d399baec2acda578a217d1aec8d7d707c71e44d", GitTreeState: ""}},
		{"v1.0.0", "79d5c5f7", "0d399baec2acda578a217d1aec8d7d707c71e44d", "clean", BuildInfo{Version: "v1.0.0+79d5c5f7", GitCommit: "0d399baec2acda578a217d1aec8d7d707c71e44d", GitTreeState: "clean"}},
	}
	for _, tt := range tests {
		version = tt.version
		metadata = tt.buildMetadata
		gitCommit = tt.gitCommit
		gitTreeState = tt.gitTreeState
		if GetBuildInfo() != tt.expected {
			t.Errorf("expected Version(%s), GitCommit(%s) and GitTreeState(%s) to be %v", tt.expected, tt.gitCommit, tt.gitTreeState, GetBuildInfo())
		}
	}
}
