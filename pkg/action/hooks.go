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
	"sync"
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

// execHookEvent executes all of the hooks for the given hook event.
func (cfg *Configuration) execHookEvent(rl *release.Release, event release.HookEvent, timeout time.Duration, parallelism int) error {
	if parallelism < 1 {
		parallelism = 1
	}

	weightedHooks := make(map[int][]*release.Hook)
	for _, h := range rl.Hooks {
		for _, e := range h.Events {
			if e == event {
				// Set default delete policy to before-hook-creation
				if h.DeletePolicies == nil || len(h.DeletePolicies) == 0 {
					// TODO(jlegrone): Only apply before-hook-creation delete policy to run to completion
					//                 resources. For all other resource types update in place if a
					//                 resource with the same name already exists and is owned by the
					//                 current release.
					h.DeletePolicies = []release.HookDeletePolicy{release.HookBeforeHookCreation}
				}
				weightedHooks[h.Weight] = append(weightedHooks[h.Weight], h)
			}
		}
	}

	weights := make([]int, 0, len(weightedHooks))
	for w := range weightedHooks {
		weights = append(weights, w)
		// sort hooks in each weighted group by name
		sort.Slice(weightedHooks[w], func(i, j int) bool {
			return weightedHooks[w][i].Name < weightedHooks[w][j].Name
		})
	}
	sort.Ints(weights)

	var mut sync.RWMutex
	for _, w := range weights {
		sem := make(chan struct{}, parallelism)
		errsChan := make(chan error)
		errs := make([]error, 0)
		for _, h := range weightedHooks[w] {
			// execute hooks in parallel (with limited parallelism enforced by semaphore)
			go func(h *release.Hook) {
				sem <- struct{}{}
				errsChan <- cfg.execHook(rl, h, &mut, timeout)
				<-sem
			}(h)
		}
		// collect errors
		for range weightedHooks[w] {
			if err := <-errsChan; err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			return errors.Errorf("%s hook event failed with %d error(s): %s", event, len(errs), joinErrors(errs))
		}
	}

	// If all hooks are successful, check the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, w := range weights {
		for _, h := range weightedHooks[w] {
			if err := cfg.deleteHookByPolicy(h, release.HookSucceeded); err != nil {
				return err
			}
		}
	}
	return nil
}

// // hooke are pre-ordered by kind, so keep order stable
// sort.Stable(hookByWeight(executingHooks))

// for _, h := range executingHooks {
// 	// Set default delete policy to before-hook-creation
// 	if h.DeletePolicies == nil || len(h.DeletePolicies) == 0 {
// 		// TODO(jlegrone): Only apply before-hook-creation delete policy to run to completion
// 		//                 resources. For all other resource types update in place if a
// 		//                 resource with the same name already exists and is owned by the
// 		//                 current release.
// 		h.DeletePolicies = []release.HookDeletePolicy{release.HookBeforeHookCreation}
// 	}

// 	if err := cfg.deleteHookByPolicy(h, release.HookBeforeHookCreation); err != nil {
// 		return err
// 	}

// 	resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest), true)
// 	if err != nil {
// 		return errors.Wrapf(err, "unable to build kubernetes object for %s hook %s", hook, h.Path)
// 	}

// execHook executes a hook.
func (cfg *Configuration) execHook(rl *release.Release, h *release.Hook, mut *sync.RWMutex, timeout time.Duration) (err error) {
	if err := cfg.deleteHookByPolicy(h, release.HookBeforeHookCreation); err != nil {
		return err
	}

	resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest), true)
	if err != nil {
		return errors.Wrapf(err, "unable to build kubernetes object for applying hook %s", h.Path)
	}

	// Record the time at which the hook was applied to the cluster
	updateHookPhase(h, mut, release.HookPhaseRunning)
	// Thread safety: exclusive lock is necessary to ensure that none of the hook structs are modified during recordRelease
	mut.Lock()
	cfg.recordRelease(rl)
	mut.Unlock()

	// As long as the implementation of WatchUntilReady does not panic, HookPhaseFailed or HookPhaseSucceeded
	// should always be set by this function. If we fail to do that for any reason, then HookPhaseUnknown is
	// the most appropriate value to surface.
	defer func() {
		if panic := recover(); panic != nil {
			updateHookPhase(h, mut, release.HookPhaseUnknown)
			err = errors.Errorf("panicked while executing hook %s", h.Path)
		}
	}()

	// Create hook resources
	if _, err = cfg.KubeClient.Create(resources); err != nil {
		updateHookPhase(h, mut, release.HookPhaseFailed)
		return errors.Wrapf(err, "warning: hook %s failed", h.Path)
	}

	// Watch hook resources until they have completed then mark hook as succeeded or failed
	if err = cfg.KubeClient.WatchUntilReady(resources, timeout); err != nil {
		updateHookPhase(h, mut, release.HookPhaseFailed)
		// If a hook is failed, check the annotation of the hook to determine whether the hook should be deleted
		// under failed condition. If so, then clear the corresponding resource object in the hook.
		if deleteHookErr := cfg.deleteHookByPolicy(h, release.HookFailed); deleteHookErr != nil {
			return deleteHookErr
		}
		return err
	}
	updateHookPhase(h, mut, release.HookPhaseSucceeded)
	return nil
}

// updateHookPhase updates the phase of a hook in a thread-safe manner.
func updateHookPhase(h *release.Hook, mut *sync.RWMutex, phase release.HookPhase) {
	// Thread safety: shared lock is sufficient because each execHook goroutine operates on a different hook
	completedAtTime := helmtime.Now()
	mut.RLock()
	startedAtTime := helmtime.Now()
	switch phase {
	case release.HookPhaseRunning:
		h.LastRun.StartedAt = startedAtTime
	case release.HookPhaseSucceeded, release.HookPhaseFailed:
		h.LastRun.CompletedAt = completedAtTime
	}
	h.LastRun.Phase = phase
	mut.RUnlock()
}

// deleteHookByPolicy deletes a hook if the hook policy instructs it to.
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
