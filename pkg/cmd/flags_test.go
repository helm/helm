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
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestWaitFlag(t *testing.T) {
	orderedCommands := []struct {
		name string
		new  func() *cobra.Command
	}{
		{name: "install", new: func() *cobra.Command { return newInstallCmd(&action.Configuration{}, io.Discard) }},
		{name: "upgrade", new: func() *cobra.Command { return newUpgradeCmd(&action.Configuration{}, io.Discard) }},
		{name: "rollback", new: func() *cobra.Command { return newRollbackCmd(&action.Configuration{}, io.Discard) }},
		{name: "uninstall", new: func() *cobra.Command { return newUninstallCmd(&action.Configuration{}, io.Discard) }},
	}
	for _, tt := range orderedCommands {
		t.Run(tt.name+" accepts ordered", func(t *testing.T) {
			cmd := tt.new()
			require.NoError(t, cmd.Flags().Set("wait", "ordered"))
			require.Equal(t, "ordered", cmd.Flags().Lookup("wait").Value.String())
		})
	}

	t.Run("bare wait selects watcher", func(t *testing.T) {
		cmd := newInstallCmd(&action.Configuration{}, io.Discard)
		require.NoError(t, cmd.ParseFlags([]string{"--wait"}))
		require.Equal(t, "watcher", cmd.Flags().Lookup("wait").Value.String())
	})

	for _, strategy := range []string{"watcher", "legacy", "hookOnly"} {
		t.Run("accepts "+strategy, func(t *testing.T) {
			cmd := newInstallCmd(&action.Configuration{}, io.Discard)
			require.NoError(t, cmd.Flags().Set("wait", strategy))
			require.Equal(t, strategy, cmd.Flags().Lookup("wait").Value.String())
		})
	}

	t.Run("plain wait rejects ordered", func(t *testing.T) {
		cmd := newTemplateCmd(&action.Configuration{}, io.Discard)
		err := cmd.Flags().Set("wait", "ordered")
		require.EqualError(t, err, `invalid argument "ordered" for "--wait" flag: invalid wait input "ordered". Valid inputs are watcher, hookOnly, and legacy`)
	})

	t.Run("omitted wait defaults to hookOnly", func(t *testing.T) {
		cmd := newInstallCmd(&action.Configuration{}, io.Discard)
		require.Equal(t, "hookOnly", cmd.Flags().Lookup("wait").Value.String())
	})
}

func TestReadinessTimeout(t *testing.T) {
	commands := []struct {
		name string
		new  func() *cobra.Command
	}{
		{name: "install", new: func() *cobra.Command { return newInstallCmd(&action.Configuration{}, io.Discard) }},
		{name: "upgrade", new: func() *cobra.Command { return newUpgradeCmd(&action.Configuration{}, io.Discard) }},
		{name: "rollback", new: func() *cobra.Command { return newRollbackCmd(&action.Configuration{}, io.Discard) }},
	}
	for _, tt := range commands {
		t.Run(tt.name+" registers readiness-timeout unset by default", func(t *testing.T) {
			cmd := tt.new()
			flag := cmd.Flags().Lookup("readiness-timeout")
			require.NotNil(t, flag)
			require.Equal(t, "0s", flag.DefValue)
			require.NoError(t, cmd.Flags().Set("readiness-timeout", "30s"))
			require.Equal(t, "30s", flag.Value.String())
		})
	}

	t.Run("other commands omit readiness-timeout", func(t *testing.T) {
		require.Nil(t, newTemplateCmd(&action.Configuration{}, io.Discard).Flags().Lookup("readiness-timeout"))
		require.Nil(t, newUninstallCmd(&action.Configuration{}, io.Discard).Flags().Lookup("readiness-timeout"))
	})
}

func TestOrderedWaitRequiresChartV3(t *testing.T) {
	t.Run("install rejects chart v1", func(t *testing.T) {
		_, _, err := executeActionCommand("install ordered-install testdata/testcharts/empty --wait=ordered")
		require.EqualError(t, err, `INSTALLATION FAILED: --wait=ordered requires chart apiVersion v3 (chart "empty" has apiVersion v1)`)
	})

	t.Run("upgrade rejects chart v1", func(t *testing.T) {
		const releaseName = "ordered-upgrade"
		relMock, ch, chartPath := prepareMockRelease(t, releaseName)
		store := storageFixture()
		require.NoError(t, store.Create(relMock(releaseName, 1, ch)))

		_, _, err := executeActionCommandC(store, fmt.Sprintf("upgrade %s %q --wait=ordered", releaseName, chartPath))
		require.EqualError(t, err, `--wait=ordered requires chart apiVersion v3 (chart "testUpgradeChart" has apiVersion v1)`)
	})
}

func outputFlagCompletionTest(t *testing.T, cmdName string) {
	t.Helper()
	releasesMockWithStatus := func(info *release.Info, hooks ...*release.Hook) []*release.Release {
		info.LastDeployed = time.Unix(1452902400, 0).UTC()
		return []*release.Release{{
			Name:      "athos",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}, {
			Name:      "porthos",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}, {
			Name:      "aramis",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}, {
			Name:      "dartagnan",
			Namespace: "gascony",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}}
	}

	tests := []cmdTestCase{{
		name:   "completion for output flag long and before arg",
		cmd:    fmt.Sprintf("__complete %s --output ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag long and after arg",
		cmd:    fmt.Sprintf("__complete %s aramis --output ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag short and before arg",
		cmd:    fmt.Sprintf("__complete %s -o ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag short and after arg",
		cmd:    fmt.Sprintf("__complete %s aramis -o ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag, no filter",
		cmd:    fmt.Sprintf("__complete %s --output jso", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
		}),
	}}
	runTestCmd(t, tests)
}

func TestPostRendererFlagSetOnce(t *testing.T) {
	cfg := action.Configuration{}
	client := action.NewInstall(&cfg)
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
	str := postRendererString{
		options: &postRendererOptions{
			renderer: &client.PostRenderer,
			settings: settings,
		},
	}
	// Set the plugin name once
	require.NoError(t, str.Set("postrenderer-v1"))

	// Set the plugin name again to the same value is not ok
	require.Error(t, str.Set("postrenderer-v1"))

	// Set the plugin name again to a different value is not ok
	require.Error(t, str.Set("cat"))
}
