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

package action

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func rollbackTestActions(t *testing.T) (*Install, *Upgrade, *Rollback) {
	config := actionConfigFixture(t)

	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "rollback-test"

	upAction := NewUpgrade(config)
	upAction.Namespace = "spaced"

	rollbackAction := NewRollback(config)
	rollbackAction.Wait = true

	return instAction, upAction, rollbackAction
}

func buildChartWithRollbackHooks(hookNamePrefix string) *chart.Chart {
	hookTmpl, _ := template.New("hook-template").Parse(`kind: ConfigMap
metadata:
  name: {{.Prefix}}-{{.Type}}
  annotations:
    "helm.sh/hook": {{.Type}}
data:
  name: value`)

	type HookTmplArgs struct {
		Prefix string
		Type   release.HookEvent
	}

	var preRollbackManifest, postRollbackManifest bytes.Buffer
	hookTmpl.Execute(&preRollbackManifest, HookTmplArgs{Prefix: hookNamePrefix, Type: release.HookPreRollback})
	hookTmpl.Execute(&postRollbackManifest, HookTmplArgs{Prefix: hookNamePrefix, Type: release.HookPostRollback})

	return buildChart(func(opts *chartOptions) {
		opts.Metadata.Name = "rollback-test"
		opts.Templates = []*chart.File{
			{Name: "templates/pre-rollback", Data: preRollbackManifest.Bytes()},
			{Name: "templates/post-rollback", Data: postRollbackManifest.Bytes()},
		}
	})
}

func TestRollbackRelease_useTargetHooks(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	instAction, upAction, rollbackAction := rollbackTestActions(t)

	vals := map[string]interface{}{}

	// create release with chart A (hooks prefixed with "chart-a")
	ctx, done := context.WithCancel(context.Background())
	relV1, err := instAction.RunWithContext(ctx, buildChartWithRollbackHooks("chart-a"), vals)
	done()
	req.NoError(err)

	// upgrade release with chart B (hooks prefixed with "chart-b")
	ctx, done = context.WithCancel(context.Background())
	relV2, err := upAction.RunWithContext(ctx, relV1.Name, buildChartWithRollbackHooks("chart-b"), vals)
	done()
	req.NoError(err)

	// rollback release to chart A (B -> A)
	err = rollbackAction.Run(relV2.Name)
	req.NoError(err)
	relV3, err := rollbackAction.cfg.Releases.Last(relV2.Name)
	req.NoError(err)

	// check hooks WERE NOT RUN from source release (chart-b)
	for _, hook := range relV2.Hooks {
		is.True(strings.HasPrefix(hook.Name, "chart-b"))
		is.True(hook.LastRun.StartedAt.IsZero())
	}

	// check hooks WERE RUN from target release (chart-a)
	for _, hook := range relV3.Hooks {
		is.True(strings.HasPrefix(hook.Name, "chart-a"))
		is.False(hook.LastRun.StartedAt.IsZero())
	}
}

func TestRollbackRelease_useSourceHooks(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	instAction, upAction, rollbackAction := rollbackTestActions(t)

	vals := map[string]interface{}{}

	// create release with chart A (hooks prefixed with "chart-a")
	ctx, done := context.WithCancel(context.Background())
	relV1, err := instAction.RunWithContext(ctx, buildChartWithRollbackHooks("chart-a"), vals)
	done()
	req.NoError(err)

	// upgrade release with chart B (hooks prefixed with "chart-b")
	ctx, done = context.WithCancel(context.Background())
	relV2, err := upAction.RunWithContext(ctx, relV1.Name, buildChartWithRollbackHooks("chart-b"), vals)
	done()
	req.NoError(err)

	// rollback release to chart A (B -> A) BUT run hooks of current release
	rollbackAction.UseSourceHooks = true
	err = rollbackAction.Run(relV2.Name)
	req.NoError(err)
	relV3, err := rollbackAction.cfg.Releases.Last(relV2.Name)
	req.NoError(err)

	// check hooks WERE RUN from source release (chart-b)
	for _, hook := range relV2.Hooks {
		is.True(strings.HasPrefix(hook.Name, "chart-b"))
		is.False(hook.LastRun.StartedAt.IsZero())
	}

	// check hooks WERE NOT RUN from target release (chart-a)
	for _, hook := range relV3.Hooks {
		is.True(strings.HasPrefix(hook.Name, "chart-a"))
		is.True(hook.LastRun.StartedAt.IsZero())
	}
}
