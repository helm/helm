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

//go:build !windows && !darwin

package helmpath

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/util/homedir"

	"helm.sh/helm/v4/pkg/helmpath/xdg"
)

const (
	appName  = "helm"
	testFile = "test.txt"
	lazy     = lazypath(appName)
)

func TestDataPath(t *testing.T) {
	expected := filepath.Join(homedir.HomeDir(), ".local", "share", appName, testFile)

	assert.Equal(t, expected, lazy.dataPath(testFile))

	t.Setenv(xdg.DataHomeEnvVar, "/tmp")

	expected = filepath.Join("/tmp", appName, testFile)

	assert.Equal(t, expected, lazy.dataPath(testFile))
}

func TestConfigPath(t *testing.T) {
	expected := filepath.Join(homedir.HomeDir(), ".config", appName, testFile)

	assert.Equal(t, expected, lazy.configPath(testFile))

	t.Setenv(xdg.ConfigHomeEnvVar, "/tmp")

	expected = filepath.Join("/tmp", appName, testFile)

	assert.Equal(t, expected, lazy.configPath(testFile))
}

func TestCachePath(t *testing.T) {
	expected := filepath.Join(homedir.HomeDir(), ".cache", appName, testFile)

	assert.Equal(t, expected, lazy.cachePath(testFile))

	t.Setenv(xdg.CacheHomeEnvVar, "/tmp")

	expected = filepath.Join("/tmp", appName, testFile)

	assert.Equal(t, expected, lazy.cachePath(testFile))
}
