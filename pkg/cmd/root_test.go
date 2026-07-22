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

package cmd

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/helmpath/xdg"
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
				t.Setenv(k, v)
			}

			_, _, err := executeActionCommand(tt.args)
			require.NoError(t, err)

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

			assert.Equal(t, tt.cachePath, helmpath.CachePath(), "expected cache path %q, got %q", tt.cachePath, helmpath.CachePath())
			assert.Equal(t, tt.configPath, helmpath.ConfigPath(), "expected config path %q, got %q", tt.configPath, helmpath.ConfigPath())
			assert.Equal(t, tt.dataPath, helmpath.DataPath(), "expected data path %q, got %q", tt.dataPath, helmpath.DataPath())
		})
	}
}

func TestUnknownSubCmd(t *testing.T) {
	_, _, err := executeActionCommand("foobar")

	assert.EqualErrorf(t, err, `unknown command "foobar" for "helm"`, "Expect unknown command error")
}

// Need the release of Cobra following 1.0 to be able to disable
// file completion on the root command.  Until then, we cannot
// because it would break 'helm help <TAB>'
//
// func TestRootFileCompletion(t *testing.T) {
// 	checkFileCompletion(t, "", false)
// }

func TestRootCmdLogger(t *testing.T) {
	args := []string{}
	buf := new(bytes.Buffer)
	actionConfig := action.NewConfiguration()
	_, err := newRootCmdWithConfig(actionConfig, buf, args, SetupLogging)
	require.NoError(t, err)

	l1 := actionConfig.Logger()
	l2 := slog.Default()

	assert.Equal(t, l2.Handler(), l1.Handler(), "expected actionConfig logger to be the slog default logger")
}
