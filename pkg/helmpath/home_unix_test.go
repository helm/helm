// Copyright The Helm Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows

package helmpath

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/helmpath/xdg"
)

func TestHelmHome(t *testing.T) {
	t.Setenv(xdg.CacheHomeEnvVar, "/cache")
	t.Setenv(xdg.ConfigHomeEnvVar, "/config")
	t.Setenv(xdg.DataHomeEnvVar, "/data")

	assert.Equal(t, "/cache/helm", CachePath())
	assert.Equal(t, "/config/helm", ConfigPath())
	assert.Equal(t, "/data/helm", DataPath())

	// test to see if lazy-loading environment variables at runtime works
	t.Setenv(xdg.CacheHomeEnvVar, "/cache2")

	assert.Equal(t, "/cache2/helm", CachePath())
}
