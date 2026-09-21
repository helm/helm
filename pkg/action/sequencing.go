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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
	"helm.sh/helm/v4/internal/release/v2/sequence"
	"helm.sh/helm/v4/pkg/kube"
)

func computeDeadline(timeout time.Duration) time.Time {
	if timeout > 0 {
		return time.Now().Add(timeout)
	}
	return time.Time{}
}

func buildManifestYAML(manifests []releaseutil.Manifest) string {
	var buf strings.Builder
	for i, manifest := range manifests {
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.WriteString(manifest.Content)
		buf.WriteString("\n")
	}
	return buf.String()
}

type sequencedDeployment struct {
	kubeClient                                                    kube.Interface
	logger                                                        *slog.Logger
	releaseName, releaseNamespace                                 string
	disableOpenAPI, serverSideApply, forceConflicts, forceReplace bool
	waitStrategy                                                  kube.WaitStrategy
	waitOptions                                                   []kube.WaitOption
	waitForJobs                                                   bool
	timeout, readinessTimeout                                     time.Duration
	deadline                                                      time.Time

	upgradeMode, upgradeCSAFieldManager, threeWayMergeForUnstructured bool
	currentResources, createdResources                                kube.ResourceList
}

func logPlanWarnings(logger *slog.Logger, plan *sequence.Plan) {
	for _, warning := range plan.Warnings {
		logger.Warn("sequencing: "+warning.Message, "chart", warning.ChartPath)
	}
}

func getWaiterFor(client kube.Interface, strategy kube.WaitStrategy, opts ...kube.WaitOption) (kube.Waiter, error) {
	if clientWithOptions, ok := client.(kube.InterfaceWaitOptions); ok {
		return clientWithOptions.GetWaiterWithOptions(strategy, opts...)
	}
	return client.GetWaiter(strategy)
}

func (s *sequencedDeployment) apply(ctx context.Context, plan *sequence.Plan) error {
	if s.logger != nil {
		logPlanWarnings(s.logger, plan)
	}
	if s.deadline.IsZero() {
		s.deadline = computeDeadline(s.timeout)
	}
	for _, batch := range plan.Batches {
		if err := s.applyBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

func (s *sequencedDeployment) applyBatch(ctx context.Context, batch sequence.Batch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(batch.Manifests) == 0 {
		return nil
	}
	target, err := s.kubeClient.Build(bytes.NewBufferString(buildManifestYAML(batch.Manifests)), !s.disableOpenAPI)
	if err != nil {
		return fmt.Errorf("building resource batch: %w", err)
	}
	if len(target) == 0 {
		return nil
	}
	if err := target.Visit(setMetadataVisitor(s.releaseName, s.releaseNamespace, true)); err != nil {
		return fmt.Errorf("setting metadata for resource batch: %w", err)
	}
	// PR2 extension point: strip manifest-level sequencing annotations here.
	if err := ctx.Err(); err != nil {
		return err
	}
	targetKeys := make(map[string]bool, len(target))
	for _, resource := range target {
		targetKeys[objectKey(resource)] = true
	}
	var matchingCurrent kube.ResourceList
	for _, resource := range s.currentResources {
		if targetKeys[objectKey(resource)] {
			matchingCurrent = append(matchingCurrent, resource)
		}
	}
	var result *kube.Result
	if s.upgradeMode || len(matchingCurrent) > 0 {
		result, err = s.kubeClient.Update(
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
		result, err = s.kubeClient.Create(target, kube.ClientCreateOptionServerSideApply(s.serverSideApply, false))
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
	return s.waitForResources(target)
}

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
		waitTimeout = time.Minute
	}
	return waitTimeout, nil
}

func (s *sequencedDeployment) waitForResources(resources kube.ResourceList) error {
	if len(resources) == 0 {
		return nil
	}
	waitTimeout, err := s.batchWaitTimeout()
	if err != nil {
		return err
	}
	waiter, err := getWaiterFor(s.kubeClient, s.waitStrategy, s.waitOptions...)
	if err != nil {
		return fmt.Errorf("getting waiter for resource batch: %w", err)
	}
	if s.waitForJobs {
		return waiter.WaitWithJobs(resources, waitTimeout)
	}
	return waiter.Wait(resources, waitTimeout)
}

func (s *sequencedDeployment) deleteRemoved(reversedPlan *sequence.Plan, removedKeys map[string]bool, waiter kube.Waiter) error {
	if s.deadline.IsZero() {
		s.deadline = computeDeadline(s.timeout)
	}
	for _, batch := range reversedPlan.Batches {
		if len(batch.Manifests) == 0 {
			continue
		}
		resources, err := s.kubeClient.Build(bytes.NewBufferString(buildManifestYAML(batch.Manifests)), false)
		if err != nil {
			return fmt.Errorf("building removed-resource batch: %w", err)
		}
		toDelete := make(kube.ResourceList, 0, len(resources))
		for _, resource := range resources {
			if removedKeys[objectKey(resource)] {
				toDelete = append(toDelete, resource)
			}
		}
		if len(toDelete) == 0 {
			continue
		}
		if _, errs := s.kubeClient.Delete(toDelete, metav1.DeletePropagationBackground); len(errs) > 0 {
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

func (s *sequencedDeployment) deleteSequencedBatches(
	reversedPlan *sequence.Plan,
	deletionPropagation metav1.DeletionPropagation,
	waiter kube.Waiter,
	keptManifests []releaseutil.Manifest,
) (kube.ResourceList, string, error) {
	if s.deadline.IsZero() {
		s.deadline = computeDeadline(s.timeout)
	}
	var report strings.Builder
	if len(keptManifests) > 0 {
		report.WriteString("These resources were kept due to the resource policy:\n")
		for _, manifest := range keptManifests {
			kind, name := "Unknown", manifest.Name
			if manifest.Head != nil {
				kind = manifest.Head.Kind
				if manifest.Head.Metadata != nil {
					name = manifest.Head.Metadata.Name
				}
			}
			fmt.Fprintf(&report, "[%s] %s\n", kind, name)
		}
	}

	var deleted kube.ResourceList
	for _, batch := range reversedPlan.Batches {
		if !s.deadline.IsZero() && time.Until(s.deadline) <= 0 {
			return deleted, report.String(), fmt.Errorf("uninstall timed out after %s before all batches were deleted", s.timeout)
		}
		if len(batch.Manifests) == 0 {
			continue
		}
		resources, err := s.kubeClient.Build(bytes.NewBufferString(buildManifestYAML(batch.Manifests)), false)
		if err != nil {
			return deleted, report.String(), fmt.Errorf("building resource batch for delete: %w", err)
		}
		owned, skipped, err := verifySequencedOwnedForDelete(resources, s.logger, s.releaseName, s.releaseNamespace)
		if err != nil {
			return deleted, report.String(), err
		}
		if skipped != "" {
			if report.Len() > 0 {
				report.WriteString("\n")
			}
			report.WriteString(skipped)
		}
		if len(owned) == 0 {
			continue
		}
		deleted = append(deleted, owned...)
		if _, errs := s.kubeClient.Delete(owned, deletionPropagation); len(errs) > 0 {
			return deleted, report.String(), fmt.Errorf("deleting resource batch: %w", joinErrors(errs, ", "))
		}
		if !batch.Wait {
			continue
		}
		if !s.deadline.IsZero() && time.Until(s.deadline) <= 0 {
			return deleted, report.String(), fmt.Errorf("uninstall timed out after %s before waiting for batch deletion", s.timeout)
		}
		waitTimeout, err := s.batchWaitTimeout()
		if err != nil {
			return deleted, report.String(), err
		}
		if err := waiter.WaitForDelete(owned, waitTimeout); err != nil {
			return deleted, report.String(), fmt.Errorf("waiting for resource batch deletion: %w", err)
		}
	}
	return deleted, report.String(), nil
}

func verifySequencedOwnedForDelete(resources kube.ResourceList, logger *slog.Logger, releaseName, releaseNamespace string) (kube.ResourceList, string, error) {
	owned, unowned, unverifiable, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
	if err != nil {
		return nil, "", fmt.Errorf("unable to verify resource ownership: %w", err)
	}
	var skipped strings.Builder
	if len(unowned) > 0 {
		fmt.Fprintf(&skipped, "%d resource(s) were not deleted because they are not owned by this release:\n", len(unowned))
		for _, info := range unowned {
			kind := info.Object.GetObjectKind().GroupVersionKind().Kind
			if logger != nil {
				logger.Warn("skipping delete of resource not owned by this release", "kind", kind, "name", info.Name, "namespace", info.Namespace, "release", releaseName)
			}
			fmt.Fprintf(&skipped, "[%s] %s\n", kind, info.Name)
		}
	}
	if len(unverifiable) > 0 {
		if skipped.Len() > 0 {
			skipped.WriteString("\n")
		}
		fmt.Fprintf(&skipped, "%d resource(s) were not deleted because their ownership could not be verified:\n", len(unverifiable))
		for _, resource := range unverifiable {
			kind := resource.Info.Object.GetObjectKind().GroupVersionKind().Kind
			if logger != nil {
				logger.Warn("skipping delete of resource because ownership could not be verified", "kind", kind, "name", resource.Info.Name, "namespace", resource.Info.Namespace, "release", releaseName, "error", resource.Err)
			}
			fmt.Fprintf(&skipped, "[%s] %s: %s\n", kind, resource.Info.Name, resource.Err)
		}
	}
	if logger != nil {
		for _, info := range owned {
			logger.Debug("deleting resource owned by this release", "kind", info.Object.GetObjectKind().GroupVersionKind().Kind, "name", info.Name, "namespace", info.Namespace, "release", releaseName)
		}
	}
	return owned, skipped.String(), nil
}

func filterSequencedManifestsToKeep(manifests []releaseutil.Manifest) (keep, remaining []releaseutil.Manifest) {
	for _, manifest := range manifests {
		if manifest.Head == nil || manifest.Head.Metadata == nil || len(manifest.Head.Metadata.Annotations) == 0 {
			remaining = append(remaining, manifest)
			continue
		}
		policy, ok := manifest.Head.Metadata.Annotations[kube.ResourcePolicyAnno]
		if !ok {
			remaining = append(remaining, manifest)
			continue
		}
		if strings.EqualFold(strings.TrimSpace(policy), kube.KeepPolicy) {
			keep = append(keep, manifest)
		}
	}
	return keep, remaining
}
