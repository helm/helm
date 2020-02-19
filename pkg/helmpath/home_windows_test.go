// Copyright The Helm Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build windows

package helmpath

import (
	"os"
	"testing"

	"helm.sh/helm/v3/pkg/helmpath/xdg"
)

func TestHelmHome(t *testing.T) {
	os.Setenv(xdg.CacheHomeEnvVar, "c:\\")
	os.Setenv(xdg.ConfigHomeEnvVar, "d:\\")
	os.Setenv(xdg.DataHomeEnvVar, "e:\\")
	isEq := func(t *testing.T, a, b string) {
		if a != b {
			t.Errorf("Expected %q, got %q", b, a)
		}
	}

	isEq(t, CachePath(), "c:\\helm")
	isEq(t, ConfigPath(), "d:\\helm")
	isEq(t, DataPath(), "e:\\helm")

	// test to see if lazy-loading environment variables at runtime works
	os.Setenv(xdg.CacheHomeEnvVar, "f:\\")

	isEq(t, CachePath(), "f:\\helm")
}
