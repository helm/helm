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
	"sort"
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

// execHook executes all of the hooks for the given hook event.
func (cfg *Configuration) execHook(rl *release.Release, hook release.HookEvent, timeout time.Duration) error {
	executingHooks := []*release.Hook{}

	for _, h := range rl.Hooks {
		for _, e := range h.Events {
			if e == hook {
				executingHooks = append(executingHooks, h)
			}
		}
	}

	// hooke are pre-ordered by kind, so keep order stable
	sort.Stable(hookByWeight(executingHooks))

	for _, h := range executingHooks {
		// Set default delete policy to before-hook-creation
		if h.DeletePolicies == nil || len(h.DeletePolicies) == 0 {
			// TODO(jlegrone): Only apply before-hook-creation delete policy to run to completion
			//                 resources. For all other resource types update in place if a
			//                 resource with the same name already exists and is owned by the
			//                 current release.
			h.DeletePolicies = []release.HookDeletePolicy{release.HookBeforeHookCreation}
		}

		if err := cfg.deleteHookByPolicy(h, release.HookBeforeHookCreation); err != nil {
			return err
		}

		resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest), true)
		if err != nil {
			return errors.Wrapf(err, "unable to build kubernetes object for %s hook %s", hook, h.Path)
		}

		// Record the time at which the hook was applied to the cluster
		h.LastRun = release.HookExecution{
			StartedAt: helmtime.Now(),
			Phase:     release.HookPhaseRunning,
		}
		cfg.recordRelease(rl)

		// As long as the implementation of WatchUntilReady does not panic, HookPhaseFailed or HookPhaseSucceeded
		// should always be set by this function. If we fail to do that for any reason, then HookPhaseUnknown is
		// the most appropriate value to surface.
		h.LastRun.Phase = release.HookPhaseUnknown

		// Create hook resources
		if _, err := cfg.KubeClient.Create(resources); err != nil {
			h.LastRun.CompletedAt = helmtime.Now()
			h.LastRun.Phase = release.HookPhaseFailed
			return errors.Wrapf(err, "warning: Hook %s %s failed", hook, h.Path)
		}

		// Watch hook resources until they have completed
		err = cfg.KubeClient.WatchUntilReady(resources, timeout)
		// Note the time of success/failure
		h.LastRun.CompletedAt = helmtime.Now()
		// Mark hook as succeeded or failed
		if err != nil {
			h.LastRun.Phase = release.HookPhaseFailed
			// If a hook is failed, check the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if err := cfg.deleteHookByPolicy(h, release.HookFailed); err != nil {
				return err
			}
			return err
		}
		h.LastRun.Phase = release.HookPhaseSucceeded
	}

	// If all hooks are successful, check the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := cfg.deleteHookByPolicy(h, release.HookSucceeded); err != nil {
			return err
		}
	}

	return nil
}

// hookByWeight is a sorter for hooks
type hookByWeight []*release.Hook

func (x hookByWeight) Len() int      { return len(x) }
func (x hookByWeight) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x hookByWeight) Less(i, j int) bool {
	if x[i].Weight == x[j].Weight {
		return x[i].Name < x[j].Name
	}
	return x[i].Weight < x[j].Weight
}

// deleteHookByPolicy deletes a hook if the hook policy instructs it to
func (cfg *Configuration) deleteHookByPolicy(h *release.Hook, policy release.HookDeletePolicy) error {
	// Never delete CustomResourceDefinitions; this could cause lots of
	// cascading garbage collection.
	if h.Kind == "CustomResourceDefinition" {
		return nil
	}
	if hookHasDeletePolicy(h, policy) {
		resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest), false)
		if err != nil {
			return errors.Wrapf(err, "unable to build kubernetes object for deleting hook %s", h.Path)
		}
		_, errs := cfg.KubeClient.Delete(resources)
		if len(errs) > 0 {
			return errors.New(joinErrors(errs))
		}
	}
	return nil
}

// hookHasDeletePolicy determines whether the defined hook deletion policy matches the hook deletion polices
// supported by helm. If so, mark the hook as one should be deleted.
func hookHasDeletePolicy(h *release.Hook, policy release.HookDeletePolicy) bool {
	for _, v := range h.DeletePolicies {
		if policy == v {
			return true
		}
	}
	return false
}
