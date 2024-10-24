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
	"sort"
	"time"

	"helm.sh/helm/v3/pkg/kube"

	"helm.sh/helm/v3/pkg/chartutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		if len(h.DeletePolicies) == 0 {
			// TODO(jlegrone): Only apply before-hook-creation delete policy to run to completion
			//                 resources. For all other resource types update in place if a
			//                 resource with the same name already exists and is owned by the
			//                 current release.
			h.DeletePolicies = []release.HookDeletePolicy{release.HookBeforeHookCreation}
		}

		if err := cfg.deleteHookByPolicy(h, release.HookBeforeHookCreation, timeout); err != nil {
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
			// If a hook is failed, check the annotation of the hook to determine if we should copy the logs client side
			if errOutputting := cfg.outputLogsByPolicy(h, rl.Namespace, release.HookOutputOnFailed); errOutputting != nil {
				// We log the error here as we want to propagate the hook failure upwards to the release object.
				log.Printf("error outputting logs for hook failure: %v", errOutputting)
			}
			// If a hook is failed, check the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if errDeleting := cfg.deleteHookByPolicy(h, release.HookFailed, timeout); err != nil {
				// We log the error here as we want to propagate the hook failure upwards to the release object.
				// This is a change in behaviour as the edge case previously would lose the hook error and only
				// raise the delete hook error.
				log.Printf("error the hook resource on hook failure: %v", errDeleting)
			}
			return err
		}
		h.LastRun.Phase = release.HookPhaseSucceeded
	}

	// If all hooks are successful, check the annotation of each hook to determine whether the hook should be deleted
	// or output should be logged under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := cfg.outputLogsByPolicy(h, rl.Namespace, release.HookOutputOnSucceeded); err != nil {
			// We log here as we still want to attempt hook resource deletion even if output logging fails.
			log.Printf("error outputting logs for hook failure: %v", err)
		}
		if err := cfg.deleteHookByPolicy(h, release.HookSucceeded, timeout); err != nil {
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
func (cfg *Configuration) deleteHookByPolicy(h *release.Hook, policy release.HookDeletePolicy, timeout time.Duration) error {
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

		// wait for resources until they are deleted to avoid conflicts
		if kubeClient, ok := cfg.KubeClient.(kube.InterfaceExt); ok {
			if err := kubeClient.WaitForDelete(resources, timeout); err != nil {
				return err
			}
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

// outputLogsByPolicy outputs a pods logs if the hook policy instructs it to
func (cfg *Configuration) outputLogsByPolicy(h *release.Hook, releaseNamespace string, policy release.HookOutputLogPolicy) error {
	if hookHasOutputLogPolicy(h, policy) {
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
	return nil
}

func (cfg *Configuration) outputContainerLogsForListOptions(namespace string, listOptions metav1.ListOptions) error {
	// TODO Helm 4: Remove this check when GetPodList and OutputContainerLogsForPodList are moved from InterfaceExt to Interface
	if kubeClient, ok := cfg.KubeClient.(kube.InterfaceExt); ok {
		podList, err := kubeClient.GetPodList(namespace, listOptions)
		if err != nil {
			return err
		}
		err = kubeClient.OutputContainerLogsForPodList(podList, namespace, log.Writer())
		return err
	}
	return nil
}

func (cfg *Configuration) deriveNamespace(h *release.Hook, namespace string) (string, error) {
	values, err := chartutil.ReadValues([]byte(h.Manifest))
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse kubernetes manifest for output logs hook %s", h.Path)
	}
	value, err := values.PathValue("metadata.namespace")
	switch err.(type) {
	case nil:
		return value.(string), nil
	case chartutil.ErrNoValue:
		return namespace, nil
	default:
		return "", errors.Wrapf(err, "unable to parse path of metadata.namespace in yaml for output logs hook %s", h.Path)
	}
}

// hookHasOutputLogPolicy determines whether the defined hook output log policy matches the hook output log policies
// supported by helm.
func hookHasOutputLogPolicy(h *release.Hook, policy release.HookOutputLogPolicy) bool {
	for _, v := range h.OutputLogPolicies {
		if policy == v {
			return true
		}
	}
	return false
}
