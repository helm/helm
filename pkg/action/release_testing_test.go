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
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
)

func releaseTestingAction(t *testing.T) *ReleaseTesting {
	config := actionConfigFixture(t)
	rtAction := NewReleaseTesting(config)
	rtAction.Namespace = "spaced"
	return rtAction
}

func TestReleaseTesting_NoFilters(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rtAction := releaseTestingAction(t)
	rel := rtReleasStub()
	rtAction.cfg.Releases.Create(rel)

	res, err := rtAction.Run(rel.Name)
	req.NoError(err)

	// The filtering implementation will not necessarily maintain the hook order so we must find the hooks by name after verifying the length
	is.Len(res.Hooks, 3)
	for _, hook := range res.Hooks {
		switch hook.Name {
		case "test-cm":
			is.Empty(hook.LastRun.Phase)
		case "finding-nemo":
			is.Equal(release.HookPhaseSucceeded, hook.LastRun.Phase)
		case "finding-dory":
			is.Equal(release.HookPhaseSucceeded, hook.LastRun.Phase)
		default:
			is.Fail("Unexpected hook: " + hook.Name)
		}
	}
}

func TestReleaseTesting_PostiveFilter(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rtAction := releaseTestingAction(t)
	rel := rtReleasStub()
	rtAction.cfg.Releases.Create(rel)
	rtAction.Filters = map[string][]string{"name": {"finding-dory"}}

	res, err := rtAction.Run(rel.Name)
	req.NoError(err)

	// The filtering implementation will not necessarily maintain the hook order so we must find the hooks by name after verifying the length
	is.Len(res.Hooks, 3)
	for _, hook := range res.Hooks {
		switch hook.Name {
		case "test-cm":
			is.Empty(hook.LastRun.Phase)
		case "finding-nemo":
			is.Empty(hook.LastRun.Phase)
		case "finding-dory":
			is.Equal(release.HookPhaseSucceeded, hook.LastRun.Phase)
		default:
			is.Fail("Unexpected hook: " + hook.Name)
		}
	}
}

func TestReleaseTesting_NegativeFilter(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rtAction := releaseTestingAction(t)
	rel := rtReleasStub()
	rtAction.cfg.Releases.Create(rel)
	rtAction.Filters = map[string][]string{"!name": {"finding-nemo"}}

	res, err := rtAction.Run(rel.Name)
	req.NoError(err)

	// The filtering implementation will not necessarily maintain the hook order so we must find the hooks by name after verifying the length
	is.Len(res.Hooks, 3)
	for _, hook := range res.Hooks {
		switch hook.Name {
		case "test-cm":
			is.Empty(hook.LastRun.Phase)
		case "finding-nemo":
			is.Empty(hook.LastRun.Phase)
		case "finding-dory":
			is.Equal(release.HookPhaseSucceeded, hook.LastRun.Phase)
		default:
			is.Fail("Unexpected hook: " + hook.Name)
		}
	}
}

func TestReleaseTesting_BothFilters(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rtAction := releaseTestingAction(t)
	rel := rtReleasStub()
	rtAction.cfg.Releases.Create(rel)
	rtAction.Filters = map[string][]string{
		"!name": {"finding-nemo"},
		"name":  {"finding-dory"},
	}

	res, err := rtAction.Run(rel.Name)
	req.NoError(err)

	// The filtering implementation will not necessarily maintain the hook order so we must find the hooks by name after verifying the length
	is.Len(res.Hooks, 3)
	for _, hook := range res.Hooks {
		switch hook.Name {
		case "test-cm":
			is.Empty(hook.LastRun.Phase)
		case "finding-nemo":
			is.Empty(hook.LastRun.Phase)
		case "finding-dory":
			is.Equal(release.HookPhaseSucceeded, hook.LastRun.Phase)
		default:
			is.Fail("Unexpected hook: " + hook.Name)
		}
	}
}

func TestReleaseTesting_ConflictingFilters(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rtAction := releaseTestingAction(t)
	rel := rtReleasStub()
	rtAction.cfg.Releases.Create(rel)
	rtAction.Filters = map[string][]string{
		"!name": {"finding-nemo"},
		"name":  {"finding-nemo"},
	}

	res, err := rtAction.Run(rel.Name)
	req.NoError(err)

	// The filtering implementation will not necessarily maintain the hook order so we must find the hooks by name after verifying the length
	is.Len(res.Hooks, 3)
	for _, hook := range res.Hooks {
		switch hook.Name {
		case "test-cm":
			is.Empty(hook.LastRun.Phase, hook.Name)
		case "finding-nemo":
			is.Empty(hook.LastRun.Phase, hook.Name)
		case "finding-dory":
			is.Empty(hook.LastRun.Phase, hook.Name)
		default:
			is.Fail("Unexpected hook: " + hook.Name)
		}
	}
}

// TestReleaseTesting_CrashedWhileFiltering verifies that https://github.com/helm/helm/issues/9398 has been corrected
// To accomplish this, it uses a fake kube client which will panic on the Create call that occurs after the initial update
// of the release. Prior to the fix, this would have lost any helm hooks that did not pass the filter.
// Due to having to crash at a specific point in the code being tested, this test is somewhat fragile and could be broken
// by changes to the underlying implementation.
func TestReleaseTesting_CrashedWhileFiltering(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rtAction := releaseTestingAction(t)
	rtAction.cfg.KubeClient = &panickingKubeClient{Interface: rtAction.cfg.KubeClient}
	rel := rtReleasStub()
	rtAction.cfg.Releases.Create(rel)
	rtAction.Filters = map[string][]string{"name": {"finding-dory"}}

	defer func() {
		if r := recover(); r != nil {
			// Retrieving the release from storage to show that it was actually altered there and not just in memory
			res, err := rtAction.cfg.Releases.Get(rel.Name, 1)
			req.NoError(err)

			// The filtering implementation will not necessarily maintain the hook order so we must find the hooks by name after verifying the length
			is.Len(res.Hooks, 3)
			for _, hook := range res.Hooks {
				switch hook.Name {
				case "test-cm":
					is.Empty(hook.LastRun.Phase)
				case "finding-nemo":
					is.Empty(hook.LastRun.Phase)
				case "finding-dory":
					is.Equal(release.HookPhaseUnknown, hook.LastRun.Phase)
				default:
					is.Fail("Unexpected hook: " + hook.Name)
				}
			}
		} else {
			is.Fail("Did not panic")
		}
	}()

	// this will panic so the return values aren't helpful
	_, _ = rtAction.Run(rel.Name)
}

func rtReleasStub() *release.Release {
	stub := releaseStub()
	stub.Hooks = append(stub.Hooks, &release.Hook{
		Name:     "finding-dory",
		Kind:     "Pod",
		Path:     "finding-dory",
		Manifest: manifestWithTestHook,
		Events: []release.HookEvent{
			release.HookTest,
		},
	})
	return stub
}

type panickingKubeClient struct{ kube.Interface }

func (f *panickingKubeClient) Create(resources kube.ResourceList) (*kube.Result, error) {
	panic("yikes")
}
