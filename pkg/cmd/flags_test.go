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

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

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

func TestWaitFlag(t *testing.T) {
	t.Run("install accepts ordered wait", func(t *testing.T) {
		cmd := newInstallCmd(&action.Configuration{}, io.Discard)
		require.NoError(t, cmd.ParseFlags([]string{"--wait=ordered"}))
		require.Equal(t, "ordered", cmd.Flags().Lookup("wait").Value.String())
	})

	t.Run("upgrade accepts ordered wait", func(t *testing.T) {
		cmd := newUpgradeCmd(&action.Configuration{}, io.Discard)
		require.NoError(t, cmd.ParseFlags([]string{"--wait=ordered"}))
		require.Equal(t, "ordered", cmd.Flags().Lookup("wait").Value.String())
	})

	t.Run("template accepts ordered wait", func(t *testing.T) {
		cmd := newTemplateCmd(&action.Configuration{}, io.Discard)
		require.NoError(t, cmd.ParseFlags([]string{"--wait=ordered"}))
		require.Equal(t, "ordered", cmd.Flags().Lookup("wait").Value.String())
	})

	t.Run("rollback rejects ordered wait", func(t *testing.T) {
		cmd := newRollbackCmd(&action.Configuration{}, io.Discard)
		err := cmd.ParseFlags([]string{"--wait=ordered"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument \"ordered\" for \"--wait\" flag")
	})

	t.Run("uninstall rejects ordered wait", func(t *testing.T) {
		cmd := newUninstallCmd(&action.Configuration{}, io.Discard)
		err := cmd.ParseFlags([]string{"--wait=ordered"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument \"ordered\" for \"--wait\" flag")
	})
}

func TestReadinessTimeout(t *testing.T) {
	t.Run("install registers readiness-timeout with one minute default", func(t *testing.T) {
		cmd := newInstallCmd(&action.Configuration{}, io.Discard)
		flag := cmd.Flags().Lookup("readiness-timeout")
		require.NotNil(t, flag)
		require.Equal(t, "1m0s", flag.DefValue)
		require.NoError(t, cmd.ParseFlags([]string{"--readiness-timeout=30s"}))
		require.Equal(t, "30s", flag.Value.String())
	})

	t.Run("upgrade registers readiness-timeout with one minute default", func(t *testing.T) {
		cmd := newUpgradeCmd(&action.Configuration{}, io.Discard)
		flag := cmd.Flags().Lookup("readiness-timeout")
		require.NotNil(t, flag)
		require.Equal(t, "1m0s", flag.DefValue)
		require.NoError(t, cmd.ParseFlags([]string{"--readiness-timeout=30s"}))
		require.Equal(t, "30s", flag.Value.String())
	})

	t.Run("template and non-applicable commands do not register readiness-timeout", func(t *testing.T) {
		require.Nil(t, newTemplateCmd(&action.Configuration{}, io.Discard).Flags().Lookup("readiness-timeout"))
		require.Nil(t, newRollbackCmd(&action.Configuration{}, io.Discard).Flags().Lookup("readiness-timeout"))
		require.Nil(t, newUninstallCmd(&action.Configuration{}, io.Discard).Flags().Lookup("readiness-timeout"))
	})

	t.Run("install rejects readiness-timeout longer than timeout", func(t *testing.T) {
		_, _, err := executeActionCommand("install timed-install testdata/testcharts/empty --wait=ordered --timeout 30s --readiness-timeout 60s")
		require.Error(t, err)
		require.Contains(t, err.Error(), "--readiness-timeout (1m0s) must not exceed --timeout (30s)")
	})

	t.Run("upgrade rejects readiness-timeout longer than timeout", func(t *testing.T) {
		releaseName := "timed-upgrade"
		relMock, ch, chartPath := prepareMockRelease(t, releaseName)
		store := storageFixture()
		require.NoError(t, store.Create(relMock(releaseName, 1, ch)))

		_, _, err := executeActionCommandC(store, fmt.Sprintf("upgrade %s '%s' --wait=ordered --timeout 30s --readiness-timeout 60s", releaseName, chartPath))
		require.Error(t, err)
		require.Contains(t, err.Error(), "--readiness-timeout (1m0s) must not exceed --timeout (30s)")
	})

	t.Run("readiness-timeout is ignored without ordered wait", func(t *testing.T) {
		_, _, err := executeActionCommand("install plain-install testdata/testcharts/empty --readiness-timeout 30s")
		require.NoError(t, err)
	})
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
	err := str.Set("postrenderer-v1")
	require.NoError(t, err)

	// Set the plugin name again to the same value is not ok
	err = str.Set("postrenderer-v1")
	require.Error(t, err)

	// Set the plugin name again to a different value is not ok
	err = str.Set("cat")
	require.Error(t, err)
}
