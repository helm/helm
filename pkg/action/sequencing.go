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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/kube"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/release/v1/sequence"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
)

// computeDeadline returns a deadline based on the given timeout duration.
// If timeout is zero or negative, it returns the zero time (no deadline).
func computeDeadline(timeout time.Duration) time.Time {
	if timeout <= 0 {
		return time.Time{}
	}
	return time.Now().Add(timeout)
}

// GroupManifestsByDirectSubchart groups manifests by the direct subchart they belong to.
// chartPath is the full path-prefix for the current chart level — at the top level
// it is the chart name (e.g. "parent"); at deeper recursion levels it is the joined
// path through "/charts/" segments (e.g. "parent/charts/sub").
// The current chart level's own manifests are returned under the empty string key "".
// Direct subcharts are keyed by their immediate directory name under
// "<chartPath>/charts/<subchart>/". Nested grandchildren are grouped under their
// direct subchart parent ("sub"), since nested sequencing is handled recursively.
func GroupManifestsByDirectSubchart(manifests []releaseutil.Manifest, chartPath string) map[string][]releaseutil.Manifest {
	result := make(map[string][]releaseutil.Manifest)
	if chartPath == "" {
		result[""] = append(result[""], manifests...)
		return result
	}

	chartsPrefix := chartPath + "/charts/"
	for _, m := range manifests {
		if !strings.HasPrefix(m.Name, chartsPrefix) {
			result[""] = append(result[""], m)
			continue
		}
		rest := m.Name[len(chartsPrefix):]
		subchartName, _, ok := strings.Cut(rest, "/")
		if !ok {
			result[""] = append(result[""], m)
			continue
		}
		result[subchartName] = append(result[subchartName], m)
	}
	return result
}

// buildManifestYAML concatenates the Content fields of the given manifests into a single
// YAML stream suitable for passing to KubeClient.Build().
func buildManifestYAML(manifests []releaseutil.Manifest) string {
	if len(manifests) == 0 {
		return ""
	}
	var buf strings.Builder
	for i, m := range manifests {
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.WriteString(m.Content)
		buf.WriteString("\n")
	}
	return buf.String()
}

// sequencedDeployment executes a sequence.Plan: ordered installation, upgrade,
// or rollback of chart resources, one batch at a time, gating each batch on
// readiness before the next starts.
type sequencedDeployment struct {
	cfg              *Configuration
	releaseName      string
	releaseNamespace string
	disableOpenAPI   bool
	serverSideApply  bool
	forceConflicts   bool
	forceReplace     bool
	waitStrategy     kube.WaitStrategy
	waitOptions      []kube.WaitOption
	waitForJobs      bool
	timeout          time.Duration
	readinessTimeout time.Duration
	deadline         time.Time // overall operation deadline

	// upgradeMode forces every batch through KubeClient.Update. When false
	// (install), a batch is still applied via Update when it intersects
	// currentResources (resources adopted with --take-ownership).
	upgradeMode bool
	// currentResources are the objects a batch may be an update of: the full
	// old-release resource list on upgrade/rollback, the toBeAdopted set on
	// install. Matched per batch by objectKey.
	currentResources       kube.ResourceList
	upgradeCSAFieldManager bool // upgrade client-side apply field manager
	// threeWayMergeForUnstructured mirrors performInstall's adoption Update
	// (TakeOwnership && !ServerSideApply); always false on upgrade/rollback.
	threeWayMergeForUnstructured bool
	createdResources             kube.ResourceList
}

// logPlanWarnings surfaces non-fatal sequencing-plan warnings to the user.
func logPlanWarnings(logger *slog.Logger, plan *sequence.Plan) {
	for _, w := range plan.Warnings {
		logger.Warn("sequencing: "+w.Message, "chart", w.ChartPath)
	}
}

// getWaiterFor returns a waiter for the given strategy, preferring the
// options-aware interface when the client supports it.
func getWaiterFor(client kube.Interface, ws kube.WaitStrategy, opts ...kube.WaitOption) (kube.Waiter, error) {
	if c, ok := client.(kube.InterfaceWaitOptions); ok {
		return c.GetWaiterWithOptions(ws, opts...)
	}
	return client.GetWaiter(ws)
}

// apply deploys the plan batch by batch, in order.
func (s *sequencedDeployment) apply(ctx context.Context, plan *sequence.Plan) error {
	for _, batch := range plan.Batches {
		if err := s.applyBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// applyBatch builds one batch's manifests, sets release metadata, strips
// Helm-internal sequencing annotations, creates or updates the resources, and
// waits for readiness. Context cancellation is honored at each stage boundary:
// before Build, before Create/Update, and before Wait.
func (s *sequencedDeployment) applyBatch(ctx context.Context, batch sequence.Batch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manifests := batch.Manifests()
	if len(manifests) == 0 {
		return nil
	}

	yaml := buildManifestYAML(manifests)
	target, err := s.cfg.KubeClient.Build(bytes.NewBufferString(yaml), !s.disableOpenAPI)
	if err != nil {
		return fmt.Errorf("building resource batch: %w", err)
	}
	if len(target) == 0 {
		return nil
	}

	if err := target.Visit(setMetadataVisitor(s.releaseName, s.releaseNamespace, true)); err != nil {
		return fmt.Errorf("setting metadata for resource batch: %w", err)
	}
	if err := stripSequencingAnnotations(target); err != nil {
		return fmt.Errorf("stripping sequencing annotations: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	targetKeys := make(map[string]bool, len(target))
	for _, r := range target {
		targetKeys[objectKey(r)] = true
	}
	var matchingCurrent kube.ResourceList
	for _, r := range s.currentResources {
		if targetKeys[objectKey(r)] {
			matchingCurrent = append(matchingCurrent, r)
		}
	}

	var result *kube.Result
	if s.upgradeMode || len(matchingCurrent) > 0 {
		result, err = s.cfg.KubeClient.Update(
			matchingCurrent,
			target,
			kube.ClientUpdateOptionForceReplace(s.forceReplace),
			kube.ClientUpdateOptionServerSideApply(s.serverSideApply, s.forceConflicts),
			kube.ClientUpdateOptionThreeWayMergeForUnstructured(s.threeWayMergeForUnstructured),
			kube.ClientUpdateOptionUpgradeClientSideFieldManager(s.upgradeCSAFieldManager),
		)
		if err != nil {
			return fmt.Errorf("updating resource batch: %w", err)
		}
	} else {
		result, err = s.cfg.KubeClient.Create(
			target,
			kube.ClientCreateOptionServerSideApply(s.serverSideApply, false))
		if err != nil {
			return fmt.Errorf("creating resource batch: %w", err)
		}
	}
	s.createdResources = append(s.createdResources, result.Created...)

	if err := ctx.Err(); err != nil {
		return err
	}

	if !batch.Wait {
		return nil
	}
	return s.waitForResources(target, batch.HasCustomReadiness)
}

// stripSequencingAnnotations removes Helm-internal sequencing annotations from
// resources before they are applied to Kubernetes. This prevents K8s API
// validation errors for annotation keys that are not valid K8s label keys.
func stripSequencingAnnotations(resources kube.ResourceList) error {
	return resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		acc, accErr := meta.Accessor(info.Object)
		if accErr != nil {
			return nil // skip non-accessible objects
		}
		annotations := acc.GetAnnotations()
		if len(annotations) == 0 {
			return nil
		}
		changed := false
		for _, key := range releaseutil.HelmInternalSequencingAnnotations() {
			if _, exists := annotations[key]; exists {
				delete(annotations, key)
				changed = true
			}
		}
		if changed {
			acc.SetAnnotations(annotations)
		}
		return nil
	})
}

// deleteRemoved deletes resources of the previous release that are absent from
// the new one, walking an already-reversed plan of the OLD release so removal
// happens in exact reverse deployment order, gating each batch on WaitForDelete
// (HIP-0025 §Sequencing order, bead 7yi). removedKeys holds the objectKey()s
// of resources to delete; batch resources outside the set are skipped.
func (s *sequencedDeployment) deleteRemoved(reversedPlan *sequence.Plan, removedKeys map[string]bool, waiter kube.Waiter) error {
	for _, batch := range reversedPlan.Batches {
		manifests := batch.Manifests()
		if len(manifests) == 0 {
			continue
		}
		resources, err := s.cfg.KubeClient.Build(bytes.NewBufferString(buildManifestYAML(manifests)), false)
		if err != nil {
			return fmt.Errorf("building removed-resource batch: %w", err)
		}
		var toDelete kube.ResourceList
		for _, r := range resources {
			if removedKeys[objectKey(r)] {
				toDelete = append(toDelete, r)
			}
		}
		if len(toDelete) == 0 {
			continue
		}
		if _, errs := s.cfg.KubeClient.Delete(toDelete, metav1.DeletePropagationBackground); errs != nil {
			return fmt.Errorf("deleting removed resources: %w", joinErrors(errs, ", "))
		}
		if !batch.Wait {
			continue
		}
		waitTimeout, err := s.batchWaitTimeout()
		if err != nil {
			return err
		}
		if err := waiter.WaitForDelete(toDelete, waitTimeout); err != nil {
			return fmt.Errorf("waiting for removed resources to be deleted: %w", err)
		}
	}
	return nil
}

// deleteRemovedFromOldRelease builds the OLD release's deployment plan from its
// stored manifest and deletes the removed resources batch-by-batch in exact
// reverse deployment order (HIP-0025 §Sequencing order, bead 7yi). When no plan
// can be built from the stored release (pre-sequencing record, unparsable
// manifest), it falls back to one unordered bulk delete with a warning.
func (s *sequencedDeployment) deleteRemovedFromOldRelease(oldRelease *release.Release, removedKeys map[string]bool, toBeDeleted kube.ResourceList, waiter kube.Waiter) error {
	oldManifests, err := sequence.ParseStoredManifests(oldRelease.Manifest)
	var oldPlan *sequence.Plan
	if err == nil {
		oldPlan, err = sequence.Build(oldRelease.Chart, oldManifests)
	}
	if err != nil {
		s.cfg.Logger().Warn("unable to build sequencing plan for previous release; deleting removed resources unordered", slog.Any("error", err))
		if _, errs := s.cfg.KubeClient.Delete(toBeDeleted, metav1.DeletePropagationBackground); errs != nil {
			return fmt.Errorf("deleting removed resources: %w", joinErrors(errs, ", "))
		}
		return nil
	}
	return s.deleteRemoved(oldPlan.Reverse(), removedKeys, waiter)
}

// batchWaitTimeout returns the effective per-batch wait timeout:
// min(readinessTimeout, time remaining until the overall deadline).
func (s *sequencedDeployment) batchWaitTimeout() (time.Duration, error) {
	waitTimeout := s.readinessTimeout
	if !s.deadline.IsZero() {
		remaining := time.Until(s.deadline)
		if remaining <= 0 {
			return 0, errors.New("overall timeout exceeded before waiting for resource batch")
		}
		if waitTimeout <= 0 || remaining < waitTimeout {
			waitTimeout = remaining
		}
	}
	if waitTimeout <= 0 {
		waitTimeout = time.Minute // safe default
	}
	return waitTimeout, nil
}

// waitForResources waits for the given resources to become ready, applying the
// per-batch and overall deadline constraints. Every batch is waited on (D1):
// leaf-group suppression was removed because it keyed on bare resource names
// (beads xnf/p1m) and made --wait=ordered weaker than --wait for terminal
// groups; Batch.LeafGroups is retained by the builder for display only.
func (s *sequencedDeployment) waitForResources(resources kube.ResourceList, hasCustomReadiness bool) error {
	if len(resources) == 0 {
		return nil
	}
	waitTimeout, err := s.batchWaitTimeout()
	if err != nil {
		return err
	}

	var waiter kube.Waiter
	waitOptions := append([]kube.WaitOption(nil), s.waitOptions...)
	if hasCustomReadiness {
		waitOptions = append(waitOptions, kube.WithCustomReadinessStatusReader())
	}
	if c, ok := s.cfg.KubeClient.(kube.InterfaceWaitOptions); ok {
		waiter, err = c.GetWaiterWithOptions(s.waitStrategy, waitOptions...)
	} else {
		waiter, err = s.cfg.KubeClient.GetWaiter(s.waitStrategy)
	}
	if err != nil {
		return fmt.Errorf("getting waiter for resource batch: %w", err)
	}

	if s.waitForJobs {
		return waiter.WaitWithJobs(resources, waitTimeout)
	}
	return waiter.Wait(resources, waitTimeout)
}
