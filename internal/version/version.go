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

package version // import "k8s.io/helm/internal/version"

import (
	hversion "k8s.io/helm/pkg/version"
)

var (
	// version is the current version of the Helm.
	// Update this whenever making a new release.
	// The version is of the format Major.Minor.Patch[-Prerelease][+BuildMetadata]
	//
	// Increment major number for new feature additions and behavioral changes.
	// Increment minor number for bug fixes and performance enhancements.
	// Increment patch number for critical fixes to existing releases.
	version = "v3.0"

	// metadata is extra build time data
	metadata = "unreleased"
	// gitCommit is the git sha1
	gitCommit = ""
	// gitTreeState is the state of the git tree
	gitTreeState = ""
)

// GetVersion returns the semver string of the version
func GetVersion() string {
	if metadata == "" {
		return version
	}
	return version + "+" + metadata
}

// GetBuildInfo returns build info
func Get() hversion.BuildInfo {
	return hversion.BuildInfo{
		Version:      GetVersion(),
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
	}
}
