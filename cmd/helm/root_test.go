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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/helmpath/xdg"
)

func TestRootCmd(t *testing.T) {
	defer resetEnv()()

	tests := []struct {
		name, args, cachePath, configPath, dataPath string
		envvars                                     map[string]string
	}{
		{
			name: "defaults",
			args: "env",
		},
		{
			name:      "with $XDG_CACHE_HOME set",
			args:      "env",
			envvars:   map[string]string{xdg.CacheHomeEnvVar: "/bar"},
			cachePath: "/bar/helm",
		},
		{
			name:       "with $XDG_CONFIG_HOME set",
			args:       "env",
			envvars:    map[string]string{xdg.ConfigHomeEnvVar: "/bar"},
			configPath: "/bar/helm",
		},
		{
			name:     "with $XDG_DATA_HOME set",
			args:     "env",
			envvars:  map[string]string{xdg.DataHomeEnvVar: "/bar"},
			dataPath: "/bar/helm",
		},
		{
			name:      "with $HELM_CACHE_HOME set",
			args:      "env",
			envvars:   map[string]string{helmpath.CacheHomeEnvVar: "/foo/helm"},
			cachePath: "/foo/helm",
		},
		{
			name:       "with $HELM_CONFIG_HOME set",
			args:       "env",
			envvars:    map[string]string{helmpath.ConfigHomeEnvVar: "/foo/helm"},
			configPath: "/foo/helm",
		},
		{
			name:     "with $HELM_DATA_HOME set",
			args:     "env",
			envvars:  map[string]string{helmpath.DataHomeEnvVar: "/foo/helm"},
			dataPath: "/foo/helm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ensure.HelmHome(t)

			for k, v := range tt.envvars {
				os.Setenv(k, v)
			}

			if _, _, err := executeActionCommand(tt.args, nil, nil); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			// NOTE(bacongobbler): we need to check here after calling ensure.HelmHome so we
			// load the proper paths after XDG_*_HOME is set
			if tt.cachePath == "" {
				tt.cachePath = filepath.Join(os.Getenv(xdg.CacheHomeEnvVar), "helm")
			}

			if tt.configPath == "" {
				tt.configPath = filepath.Join(os.Getenv(xdg.ConfigHomeEnvVar), "helm")
			}

			if tt.dataPath == "" {
				tt.dataPath = filepath.Join(os.Getenv(xdg.DataHomeEnvVar), "helm")
			}

			if helmpath.CachePath() != tt.cachePath {
				t.Errorf("expected cache path %q, got %q", tt.cachePath, helmpath.CachePath())
			}
			if helmpath.ConfigPath() != tt.configPath {
				t.Errorf("expected config path %q, got %q", tt.configPath, helmpath.ConfigPath())
			}
			if helmpath.DataPath() != tt.dataPath {
				t.Errorf("expected data path %q, got %q", tt.dataPath, helmpath.DataPath())
			}
		})
	}
}

func TestUnknownSubCmd(t *testing.T) {
	_, _, err := executeActionCommand("foobar", nil, nil)

	if err == nil || err.Error() != `unknown command "foobar" for "helm"` {
		t.Errorf("Expect unknown command error, got %q", err)
	}
}

// Need the release of Cobra following 1.0 to be able to disable
// file completion on the root command.  Until then, we cannot
// because it would break 'helm help <TAB>'
//
// func TestRootFileCompletion(t *testing.T) {
// 	checkFileCompletion(t, "", false)
// }
