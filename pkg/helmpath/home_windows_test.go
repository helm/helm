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

//go:build windows

package helmpath

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/helmpath/xdg"
)

func TestHelmHome(t *testing.T) {
	os.Setenv(xdg.CacheHomeEnvVar, "c:\\")
	os.Setenv(xdg.ConfigHomeEnvVar, "d:\\")
	os.Setenv(xdg.DataHomeEnvVar, "e:\\")

	assert.Equal(t, "c:\\helm", CachePath())
	assert.Equal(t, "d:\\helm", ConfigPath())
	assert.Equal(t, "e:\\helm", DataPath())

	// test to see if lazy-loading environment variables at runtime works
	os.Setenv(xdg.CacheHomeEnvVar, "f:\\")

	assert.Equal(t, "f:\\helm", CachePath())
}
