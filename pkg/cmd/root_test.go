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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/logging"
	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/helmpath/xdg"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
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
	origDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origDefault) })

	args := []string{}
	buf := new(bytes.Buffer)
	actionConfig := action.NewConfiguration()
	_, err := newRootCmdWithConfig(actionConfig, buf, args, loggerFromSetup(SetupLogging))
	require.NoError(t, err)

	l1 := actionConfig.Logger()
	l2 := slog.Default()

	assert.Equal(t, l2.Handler(), l1.Handler(), "expected actionConfig logger to be the slog default logger")
}

func TestRootCmdWithLoggerLeavesDefaultUntouched(t *testing.T) {
	origDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origDefault) })
	sentinel := slog.New(slog.DiscardHandler)
	slog.SetDefault(sentinel)

	var gotDebug bool
	injected := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
	actionConfig := action.NewConfiguration()
	_, err := newRootCmdWithConfig(actionConfig, io.Discard, []string{"--debug"}, func(debug bool) *slog.Logger {
		gotDebug = debug
		return injected
	})
	require.NoError(t, err)

	assert.True(t, gotDebug, "expected the --debug flag value to be passed to the logger constructor")
	assert.Same(t, sentinel, slog.Default(), "expected the slog default logger to be left untouched")
	assert.Equal(t, injected.Handler(), actionConfig.Logger().Handler(), "expected actionConfig to use the injected logger")
}

func TestRootCmdWithNilLogger(t *testing.T) {
	origDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origDefault) })
	sentinel := slog.New(slog.DiscardHandler)
	slog.SetDefault(sentinel)

	tests := map[string]func(bool) *slog.Logger{
		"nil constructor":        nil,
		"constructor return nil": func(bool) *slog.Logger { return nil },
	}
	for name, newLogger := range tests {
		t.Run(name, func(t *testing.T) {
			actionConfig := action.NewConfiguration()
			_, err := newRootCmdWithConfig(actionConfig, io.Discard, []string{}, newLogger)
			require.NoError(t, err)

			assert.Same(t, sentinel, slog.Default(), "expected the slog default logger to be left untouched")
			assert.IsType(t, &logging.DebugCheckHandler{}, actionConfig.Logger().Handler(), "expected the Helm CLI logger as the fallback")
		})
	}
}

func TestRootCmdWithLoggerRoutesCommandLogs(t *testing.T) {
	defer resetEnv()()

	// A record logged through the slog default logger instead of the injected
	// one ends up in globalBuf.
	origDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origDefault) })
	var globalBuf bytes.Buffer
	sentinel := slog.New(slog.NewTextHandler(&globalBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(sentinel)

	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	tests := []struct {
		name    string
		cmd     string
		wantLog string
	}{
		{
			name:    "install --wait=true deprecation warning",
			cmd:     "install aeneas testdata/testcharts/empty --wait=true",
			wantLog: "--wait=true is deprecated",
		},
		{
			name:    "template --dry-run deprecation warning",
			cmd:     "template testdata/testcharts/empty --dry-run",
			wantLog: "--dry-run is deprecated",
		},
		{
			name:    "upgrade --install debug output",
			cmd:     "upgrade --install funny-bunny testdata/testcharts/empty --dry-run=true",
			wantLog: "Original chart version",
		},
		{
			name:    "search repo debug output",
			cmd:     "search repo alpine --repository-config " + repoFile + " --repository-cache " + repoCache,
			wantLog: "original chart version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalBuf.Reset()
			var injectedBuf bytes.Buffer
			injected := slog.New(slog.NewTextHandler(&injectedBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			// Mirror action.Configuration.Init, which hands the configuration's
			// logger to the storage driver.
			mem := driver.NewMemory()
			mem.SetLogger(injected.Handler())
			store := storage.Init(mem)

			_, _, err := executeActionCommandWithLoggerC(store, nil, func(bool) *slog.Logger { return injected }, tt.cmd)
			require.NoError(t, err)

			assert.Contains(t, injectedBuf.String(), tt.wantLog)
			assert.NotContains(t, globalBuf.String(), tt.wantLog, "expected the record not to go through the slog default logger")
			assert.Same(t, sentinel, slog.Default(), "expected the slog default logger to be left untouched")
		})
	}
}

func TestNewLogger(t *testing.T) {
	ctx := t.Context()

	assert.False(t, NewLogger(false).Enabled(ctx, slog.LevelDebug), "expected debug records to be dropped without --debug")
	assert.True(t, NewLogger(false).Enabled(ctx, slog.LevelWarn))
	assert.True(t, NewLogger(true).Enabled(ctx, slog.LevelDebug), "expected debug records with --debug")
}
