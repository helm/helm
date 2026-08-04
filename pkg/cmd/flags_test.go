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
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/postrenderer"
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

func TestPostRendererFlagAllowsMultiple(t *testing.T) {
	cfg := action.Configuration{}
	client := action.NewInstall(&cfg)
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
	str := postRendererNameFlag{
		postRendererChainOptions: &postRendererChainOptions{
			renderer: &client.PostRenderer,
			settings: settings,
		},
	}
	// Setting the plugin name once is ok
	require.NoError(t, str.Set("postrenderer-v1"))
	require.NotNil(t, client.PostRenderer)

	// Setting a second plugin name chains it after the first
	require.NoError(t, str.Set("postrenderer-v1-second"))
	require.NotNil(t, client.PostRenderer)
	require.IsType(t, &postrenderer.Chain{}, client.PostRenderer)
}

func TestPostRendererArgs_WithoutPrecedingRendererErrors(t *testing.T) {
	cfg := action.Configuration{}
	client := action.NewInstall(&cfg)
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
	args := postRendererArgsFlag{
		options: &postRendererChainOptions{
			renderer: &client.PostRenderer,
			settings: settings,
		},
	}
	require.Error(t, args.Set("ARG1"))
}

func TestPostRendererChain_RunsInOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: test uses a sed-based plugin")
	}
	cfg := action.Configuration{}
	client := action.NewInstall(&cfg)
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"

	options := &postRendererChainOptions{
		renderer: &client.PostRenderer,
		settings: settings,
	}
	str := postRendererNameFlag{postRendererChainOptions: options}
	argsFlag := postRendererArgsFlag{options: options}

	// First renderer: FOOTEST -> BARTEST
	require.NoError(t, str.Set("postrenderer-v1"))
	// Second renderer: BARTEST -> BAZTEST
	require.NoError(t, str.Set("postrenderer-v1-second"))

	require.NotNil(t, client.PostRenderer)

	out, err := client.PostRenderer.Run(bytes.NewBufferString("FOOTEST"))
	require.NoError(t, err)
	require.Contains(t, out.String(), "BAZTEST")

	// Args apply to the most recently added renderer (the second one)
	require.NoError(t, argsFlag.Set("CUSTOM"))
	out, err = client.PostRenderer.Run(bytes.NewBufferString("FOOTEST"))
	require.NoError(t, err)
	require.Contains(t, out.String(), "CUSTOM")
}
