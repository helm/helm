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
