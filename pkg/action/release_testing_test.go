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
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/release"
)

func releaseTestingAction(t *testing.T) *ReleaseTesting {
	config := actionConfigFixture(t)
	testAction := NewReleaseTesting(config)
	return testAction
}

func TestReleaseTesting(t *testing.T) {
	is := assert.New(t)

	testAction := releaseTestingAction(t)

	rel := releaseStub()
	testAction.cfg.Releases.Create(rel)
	expectedConfigMapHook := release.Hook{
		Name: "test-cm",
		Kind: "ConfigMap",
		Path: "test-cm",
		LastRun: release.HookExecution{
			Phase: "",
			Log:   (*release.HookLog)(nil),
		},
	}
	expectedPodHook := release.Hook{
		Name: "finding-nemo",
		Kind: "Pod",
		Path: "finding-nemo",
		LastRun: release.HookExecution{
			Phase: "Succeeded",
			Log:   pointerTo(release.HookLog("example test pod log output")),
		},
	}
	res, err := testAction.Run(rel.Name)
	is.NoError(err)
	is.Equal("angry-panda", res.Name)
	is.Len(res.Hooks, 2, "The action seems to have changed the number of hooks on the release.")
	checkHook(t, expectedConfigMapHook, res.Hooks[0])
	checkHook(t, expectedPodHook, res.Hooks[1])
}

func checkHook(t *testing.T, expected release.Hook, actual *release.Hook) {
	t.Helper()
	is := assert.New(t)
	is.Equal(expected.Name, actual.Name)
	is.Equal(expected.Kind, actual.Kind)
	is.Equal(expected.Path, actual.Path)
	is.Equal(expected.LastRun.Phase, actual.LastRun.Phase)
	is.Equal(expected.LastRun.Log, actual.LastRun.Log)
	// Cannot expect start and completion times because they cannot be predicted.
}

func pointerTo(x release.HookLog) *release.HookLog {
	return &x
}
