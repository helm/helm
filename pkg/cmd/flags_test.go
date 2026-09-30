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
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
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

func TestWaitValueLogsToInjectedLogger(t *testing.T) {
	tests := []struct {
		value   string
		want    kube.WaitStrategy
		wantLog string
	}{
		{value: "true", want: kube.StatusWatcherStrategy, wantLog: "--wait=true is deprecated"},
		{value: "false", want: kube.HookOnlyStrategy, wantLog: "--wait=false is deprecated"},
		{value: "legacy", want: kube.LegacyStrategy},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			var logBuf bytes.Buffer
			var ws kube.WaitStrategy
			v := newWaitValue(kube.HookOnlyStrategy, &ws, slog.New(slog.NewTextHandler(&logBuf, nil)))
			assert.Equal(t, kube.HookOnlyStrategy, ws)

			require.NoError(t, v.Set(tt.value))
			assert.Equal(t, tt.want, ws)
			assert.Equal(t, string(tt.want), v.String())
			if tt.wantLog == "" {
				assert.Empty(t, logBuf.String())
			} else {
				assert.Contains(t, logBuf.String(), tt.wantLog)
			}
		})
	}
}

func TestAddWaitFlagLogsToDefaultLogger(t *testing.T) {
	origDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origDefault) })
	var logBuf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))

	cmd := &cobra.Command{Use: "test"}
	var ws kube.WaitStrategy
	AddWaitFlag(cmd, &ws)
	require.NoError(t, cmd.Flags().Set("wait", "true"))

	assert.Equal(t, kube.StatusWatcherStrategy, ws)
	assert.Contains(t, logBuf.String(), "--wait=true is deprecated")
}
