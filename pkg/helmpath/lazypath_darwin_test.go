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

//go:build darwin

package helmpath

import (
	"os"
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
	os.Unsetenv(xdg.DataHomeEnvVar)

	expected := filepath.Join(homedir.HomeDir(), "Library", appName, testFile)
	assert.Equal(t, expected, lazy.dataPath(testFile))

	tmpDir := t.TempDir()
	t.Setenv(xdg.DataHomeEnvVar, tmpDir)

	expected = filepath.Join(tmpDir, appName, testFile)
	assert.Equal(t, expected, lazy.dataPath(testFile))
}

func TestConfigPath(t *testing.T) {
	os.Unsetenv(xdg.ConfigHomeEnvVar)

	expected := filepath.Join(homedir.HomeDir(), "Library", "Preferences", appName, testFile)
	assert.Equal(t, expected, lazy.configPath(testFile))

	tmpDir := t.TempDir()
	t.Setenv(xdg.ConfigHomeEnvVar, tmpDir)

	expected = filepath.Join(tmpDir, appName, testFile)
	assert.Equal(t, expected, lazy.configPath(testFile))
}

func TestCachePath(t *testing.T) {
	os.Unsetenv(xdg.CacheHomeEnvVar)

	expected := filepath.Join(homedir.HomeDir(), "Library", "Caches", appName, testFile)
	assert.Equal(t, expected, lazy.cachePath(testFile))

	tmpDir := t.TempDir()
	t.Setenv(xdg.CacheHomeEnvVar, tmpDir)

	expected = filepath.Join(tmpDir, appName, testFile)
	assert.Equal(t, expected, lazy.cachePath(testFile))
}
