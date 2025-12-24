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
	"flag"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

var (
	// version is the current version of Helm.
	// Update this whenever making a new release.
	// The version is of the format Major.Minor.Patch[-Prerelease][+BuildMetadata]
	//
	// Increment major number for new feature additions and behavioral changes.
	// Increment minor number for bug fixes and performance enhancements.
	version = "v4.1"

	// metadata is extra build time data
	metadata = ""
	// gitCommit is the git sha1
	gitCommit = ""
	// gitTreeState is the state of the git tree
	gitTreeState = ""
)

const (
	kubeClientGoVersionTesting = "v1.20"
)

// BuildInfo describes the compile time information.
type BuildInfo struct {
	// Version is the current semver.
	Version string `json:"version,omitempty"`
	// GitCommit is the git sha1.
	GitCommit string `json:"git_commit,omitempty"`
	// GitTreeState is the state of the git tree.
	GitTreeState string `json:"git_tree_state,omitempty"`
	// GoVersion is the version of the Go compiler used.
	GoVersion string `json:"go_version,omitempty"`
	// KubeClientVersion is the version of client-go Helm was build with
	KubeClientVersion string `json:"kube_client_version"`
}

// GetVersion returns the semver string of the version
func GetVersion() string {
	if metadata == "" {
		return version
	}
	return version + "+" + metadata
}

// GetUserAgent returns a user agent for user with an HTTP client
func GetUserAgent() string {
	return "Helm/" + strings.TrimPrefix(GetVersion(), "v")
}

// Get returns build info
func Get() BuildInfo {

	makeKubeClientVersionString := func() string {
		// Test builds don't include debug info / module info
		// (And even if they did, we probably want a stable version during tests anyway)
		// Return a default value for test builds
		if testing.Testing() {
			return kubeClientGoVersionTesting
		}

		vstr, err := K8sIOClientGoModVersion()
		if err != nil {
			slog.Error("failed to retrieve k8s.io/client-go version", slog.Any("error", err))
			return ""
		}

		v, err := semver.NewVersion(vstr)
		if err != nil {
			slog.Error("unable to parse k8s.io/client-go version", slog.String("version", vstr), slog.Any("error", err))
			return ""
		}

		kubeClientVersionMajor := v.Major() + 1
		kubeClientVersionMinor := v.Minor()

		return fmt.Sprintf("v%d.%d", kubeClientVersionMajor, kubeClientVersionMinor)
	}

	v := BuildInfo{
		Version:           GetVersion(),
		GitCommit:         gitCommit,
		GitTreeState:      gitTreeState,
		GoVersion:         runtime.Version(),
		KubeClientVersion: makeKubeClientVersionString(),
	}

	// HACK(bacongobbler): strip out GoVersion during a test run for consistent test output
	if flag.Lookup("test.v") != nil {
		v.GoVersion = ""
	}
	return v
}
