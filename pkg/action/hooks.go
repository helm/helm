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
	"fmt"
	"log"
	"slices"
	"sort"
	"time"

	"helm.sh/helm/v4/pkg/kube"

	"go.yaml.in/yaml/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	release "helm.sh/helm/v4/pkg/release/v1"
)

// execHook executes all of the hooks for the given hook event.
func (cfg *Configuration) execHook(rl *release.Release, hook release.HookEvent,
	waitStrategy kube.WaitStrategy, waitOptions []kube.WaitOption,
	timeout time.Duration, serverSideApply bool) error {

	shutdown, err := cfg.execHookWithDelayedShutdown(rl, hook, waitStrategy, waitOptions, timeout, serverSideApply)
	if shutdown == nil {
		return err
	}
	if err != nil {
		if err := shutdown(); err != nil {
			return err
		}
		return err
	}
	return shutdown()
}

type ExecuteShutdownFunc = func() error

func shutdownNoOp() error {
	return nil
}

// execHookWithDelayedShutdown executes all of the hooks for the given hook event and returns a shutdownHook function to trigger deletions after doing other things like e.g. retrieving logs.
func (cfg *Configuration) execHookWithDelayedShutdown(rl *release.Release, hook release.HookEvent,
	waitStrategy kube.WaitStrategy, waitOptions []kube.WaitOption, timeout time.Duration,
	serverSideApply bool) (ExecuteShutdownFunc, error) {

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

	for i, h := range executingHooks {
		// Set default delete policy to before-hook-creation
		cfg.hookSetDeletePolicy(h)

		if err := cfg.deleteHookByPolicy(h, release.HookBeforeHookCreation, waitStrategy, waitOptions, timeout); err != nil {
			return shutdownNoOp, err
		}

		resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest), true)
		if err != nil {
			return shutdownNoOp, fmt.Errorf("unable to build kubernetes object for %s hook %s: %w", hook, h.Path, err)
		}

		// Record the time at which the hook was applied to the cluster
		h.LastRun = release.HookExecution{
			StartedAt: time.Now(),
			Phase:     release.HookPhaseRunning,
		}
		cfg.recordRelease(rl)

		// As long as the implementation of WatchUntilReady does not panic, HookPhaseFailed or HookPhaseSucceeded
		// should always be set by this function. If we fail to do that for any reason, then HookPhaseUnknown is
		// the most appropriate value to surface.
		h.LastRun.Phase = release.HookPhaseUnknown

		// Create hook resources
		if _, err := cfg.KubeClient.Create(
			resources,
			kube.ClientCreateOptionServerSideApply(serverSideApply, false)); err != nil {
			h.LastRun.CompletedAt = time.Now()
			h.LastRun.Phase = release.HookPhaseFailed
			return shutdownNoOp, fmt.Errorf("warning: Hook %s %s failed: %w", hook, h.Path, err)
		}

		var waiter kube.Waiter
		if c, supportsOptions := cfg.KubeClient.(kube.InterfaceWaitOptions); supportsOptions {
			waiter, err = c.GetWaiterWithOptions(waitStrategy, waitOptions...)
		} else {
			waiter, err = cfg.KubeClient.GetWaiter(waitStrategy)
		}
		if err != nil {
			return shutdownNoOp, fmt.Errorf("unable to get waiter: %w", err)
		}
		// Watch hook resources until they have completed
		err = waiter.WatchUntilReady(resources, timeout)
		// Note the time of success/failure
		h.LastRun.CompletedAt = time.Now()
		// Mark hook as succeeded or failed
		if err != nil {
			h.LastRun.Phase = release.HookPhaseFailed
			// If a hook is failed, check the annotation of the hook to determine if we should copy the logs client side
			if errOutputting := cfg.outputLogsByPolicy(h, rl.Namespace, release.HookOutputOnFailed); errOutputting != nil {
				// We log the error here as we want to propagate the hook failure upwards to the release object.
				log.Printf("error outputting logs for hook failure: %v", errOutputting)
			}
			// If a hook is failed, check the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			return func() error {
				if errDeleting := cfg.deleteHookByPolicy(h, release.HookFailed, waitStrategy, waitOptions, timeout); errDeleting != nil {
					// We log the error here as we want to propagate the hook failure upwards to the release object.
					log.Printf("error deleting the hook resource on hook failure: %v", errDeleting)
				}

				// If a hook is failed, check the annotation of the previous successful hooks to determine whether the hooks
				// should be deleted under succeeded condition.
				if err := cfg.deleteHooksByPolicy(executingHooks[0:i], release.HookSucceeded, waitStrategy, waitOptions, timeout); err != nil {
					return err
				}
				return err
			}, err
		}
		h.LastRun.Phase = release.HookPhaseSucceeded
	}

	return func() error {
		// If all hooks are successful, check the annotation of each hook to determine whether the hook should be deleted
		// or output should be logged under succeeded condition. If so, then clear the corresponding resource object in each hook
		for i := len(executingHooks) - 1; i >= 0; i-- {
			h := executingHooks[i]
			if err := cfg.outputLogsByPolicy(h, rl.Namespace, release.HookOutputOnSucceeded); err != nil {
				// We log here as we still want to attempt hook resource deletion even if output logging fails.
				log.Printf("error outputting logs for hook failure: %v", err)
			}
			if err := cfg.deleteHookByPolicy(h, release.HookSucceeded, waitStrategy, waitOptions, timeout); err != nil {
				return err
			}
		}
		return nil
	}, nil
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
func (cfg *Configuration) deleteHookByPolicy(h *release.Hook, policy release.HookDeletePolicy,
	waitStrategy kube.WaitStrategy, waitOptions []kube.WaitOption, timeout time.Duration) error {

	// Never delete CustomResourceDefinitions; this could cause lots of
	// cascading garbage collection.
	if h.Kind == "CustomResourceDefinition" {
		return nil
	}
	if cfg.hookHasDeletePolicy(h, policy) {
		resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest), false)
		if err != nil {
			return fmt.Errorf("unable to build kubernetes object for deleting hook %s: %w", h.Path, err)
		}
		_, errs := cfg.KubeClient.Delete(resources, metav1.DeletePropagationBackground)
		if len(errs) > 0 {
			return joinErrors(errs, "; ")
		}

		var waiter kube.Waiter
		if c, supportsOptions := cfg.KubeClient.(kube.InterfaceWaitOptions); supportsOptions {
			waiter, err = c.GetWaiterWithOptions(waitStrategy, waitOptions...)
		} else {
			waiter, err = cfg.KubeClient.GetWaiter(waitStrategy)
		}
		if err != nil {
			return err
		}
		if err := waiter.WaitForDelete(resources, timeout); err != nil {
			return err
		}
	}
	return nil
}

// deleteHooksByPolicy deletes all hooks if the hook policy instructs it to
func (cfg *Configuration) deleteHooksByPolicy(hooks []*release.Hook, policy release.HookDeletePolicy,
	waitStrategy kube.WaitStrategy, waitOptions []kube.WaitOption, timeout time.Duration) error {

	for _, h := range hooks {
		if err := cfg.deleteHookByPolicy(h, policy, waitStrategy, waitOptions, timeout); err != nil {
			return err
		}
	}

	return nil
}

// hookHasDeletePolicy determines whether the defined hook deletion policy matches the hook deletion polices
// supported by helm. If so, mark the hook as one should be deleted.
func (cfg *Configuration) hookHasDeletePolicy(h *release.Hook, policy release.HookDeletePolicy) bool {
	cfg.mutex.Lock()
	defer cfg.mutex.Unlock()
	return slices.Contains(h.DeletePolicies, policy)
}

// hookSetDeletePolicy determines whether the defined hook deletion policy matches the hook deletion polices
// supported by helm. If so, mark the hook as one should be deleted.
func (cfg *Configuration) hookSetDeletePolicy(h *release.Hook) {
	cfg.mutex.Lock()
	defer cfg.mutex.Unlock()
	if len(h.DeletePolicies) == 0 {
		// TODO(jlegrone): Only apply before-hook-creation delete policy to run to completion
		//                 resources. For all other resource types update in place if a
		//                 resource with the same name already exists and is owned by the
		//                 current release.
		h.DeletePolicies = []release.HookDeletePolicy{release.HookBeforeHookCreation}
	}
}

// outputLogsByPolicy outputs a pods logs if the hook policy instructs it to
func (cfg *Configuration) outputLogsByPolicy(h *release.Hook, releaseNamespace string, policy release.HookOutputLogPolicy) error {
	if !hookHasOutputLogPolicy(h, policy) {
		return nil
	}
	namespace, err := cfg.deriveNamespace(h, releaseNamespace)
	if err != nil {
		return err
	}
	switch h.Kind {
	case "Job":
		return cfg.outputContainerLogsForListOptions(namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", h.Name)})
	case "Pod":
		return cfg.outputContainerLogsForListOptions(namespace, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", h.Name)})
	default:
		return nil
	}
}

func (cfg *Configuration) outputContainerLogsForListOptions(namespace string, listOptions metav1.ListOptions) error {
	podList, err := cfg.KubeClient.GetPodList(namespace, listOptions)
	if err != nil {
		return err
	}

	return cfg.KubeClient.OutputContainerLogsForPodList(podList, namespace, cfg.HookOutputFunc)
}

func (cfg *Configuration) deriveNamespace(h *release.Hook, namespace string) (string, error) {
	tmp := struct {
		Metadata struct {
			Namespace string
		}
	}{}
	err := yaml.Unmarshal([]byte(h.Manifest), &tmp)
	if err != nil {
		return "", fmt.Errorf("unable to parse metadata.namespace from kubernetes manifest for output logs hook %s: %w", h.Path, err)
	}
	if tmp.Metadata.Namespace == "" {
		return namespace, nil
	}
	return tmp.Metadata.Namespace, nil
}

// hookHasOutputLogPolicy determines whether the defined hook output log policy matches the hook output log policies
// supported by helm.
func hookHasOutputLogPolicy(h *release.Hook, policy release.HookOutputLogPolicy) bool {
	return slices.Contains(h.OutputLogPolicies, policy)
}
