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
// +build !windows,!darwin

package helmpath

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/util/homedir"

	"helm.sh/helm/v3/pkg/helmpath/xdg"
)

const (
	appName  = "helm"
	testFile = "test.txt"
	lazy     = lazypath(appName)
)

func TestDataPath(t *testing.T) {
	os.Unsetenv(xdg.DataHomeEnvVar)

	expected := filepath.Join(homedir.HomeDir(), ".local", "share", appName, testFile)

	if lazy.dataPath(testFile) != expected {
		t.Errorf("expected '%s', got '%s'", expected, lazy.dataPath(testFile))
	}

	os.Setenv(xdg.DataHomeEnvVar, "/tmp")

	expected = filepath.Join("/tmp", appName, testFile)

	if lazy.dataPath(testFile) != expected {
		t.Errorf("expected '%s', got '%s'", expected, lazy.dataPath(testFile))
	}
}

func TestConfigPath(t *testing.T) {
	os.Unsetenv(xdg.ConfigHomeEnvVar)

	expected := filepath.Join(homedir.HomeDir(), ".config", appName, testFile)

	if lazy.configPath(testFile) != expected {
		t.Errorf("expected '%s', got '%s'", expected, lazy.configPath(testFile))
	}

	os.Setenv(xdg.ConfigHomeEnvVar, "/tmp")

	expected = filepath.Join("/tmp", appName, testFile)

	if lazy.configPath(testFile) != expected {
		t.Errorf("expected '%s', got '%s'", expected, lazy.configPath(testFile))
	}
}

func TestCachePath(t *testing.T) {
	os.Unsetenv(xdg.CacheHomeEnvVar)

	expected := filepath.Join(homedir.HomeDir(), ".cache", appName, testFile)

	if lazy.cachePath(testFile) != expected {
		t.Errorf("expected '%s', got '%s'", expected, lazy.cachePath(testFile))
	}

	os.Setenv(xdg.CacheHomeEnvVar, "/tmp")

	expected = filepath.Join("/tmp", appName, testFile)

	if lazy.cachePath(testFile) != expected {
		t.Errorf("expected '%s', got '%s'", expected, lazy.cachePath(testFile))
	}
}
