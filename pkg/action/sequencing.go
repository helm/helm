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
	"fmt"
	"log/slog"
	"strings"
	"time"

	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"

	"k8s.io/apimachinery/pkg/api/meta"
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
// The current chart's own manifests are returned under the empty string key "".
// Subcharts are keyed by their immediate name under the first `<chartName>/charts/<subchart>/`
// segment found in the manifest source path.
// Nested subcharts (e.g., `<chartName>/charts/sub/charts/nested/`) are grouped under
// the direct subchart name ("sub"), since nested sequencing is handled recursively.
func GroupManifestsByDirectSubchart(manifests []releaseutil.Manifest, chartName string) map[string][]releaseutil.Manifest {
	result := make(map[string][]releaseutil.Manifest)
	if chartName == "" {
		// Fallback: assign everything to parent
		result[""] = append(result[""], manifests...)
		return result
	}

	chartsPrefix := chartName + "/charts/"
	for _, m := range manifests {
		idx := strings.Index(m.Name, chartsPrefix)
		if idx < 0 {
			// Parent chart manifest
			result[""] = append(result[""], m)
			continue
		}
		// Extract the direct subchart name (first segment after "<chartName>/charts/")
		rest := m.Name[idx+len(chartsPrefix):]
		// rest is like "subchart1/templates/deploy.yaml" or "subchart1/charts/nested/..."
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			// Unlikely: a file directly under charts/ with no subdirectory
			result[""] = append(result[""], m)
			continue
		}
		subchartName := rest[:slashIdx]
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

// sequencedDeployment performs ordered installation or upgrade of chart resources.
// It handles the two-level DAG: first subchart ordering, then resource-group
// ordering within each chart level.
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

	// Upgrade-specific fields. When upgradeMode is true, createAndWait delegates
	// to updateAndWait which calls KubeClient.Update() instead of Create().
	upgradeMode            bool
	currentResources       kube.ResourceList // full set of old (current) resources
	upgradeCSAFieldManager bool              // upgrade client-side apply field manager
	createdResources       kube.ResourceList
}

// deployChartLevel deploys all resources for a single chart level in sequenced order.
// It first handles subcharts in dependency order (recursively), then deploys the
// parent chart's own resource-group batches.
func (s *sequencedDeployment) deployChartLevel(ctx context.Context, chrt *chartv2.Chart, manifests []releaseutil.Manifest) error {
	// Group manifests by direct subchart
	grouped := GroupManifestsByDirectSubchart(manifests, chrt.Name())

	// Build subchart DAG and deploy in topological order
	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return fmt.Errorf("building subchart DAG for %s: %w", chrt.Name(), err)
	}

	batches, err := dag.GetBatches()
	if err != nil {
		return fmt.Errorf("getting subchart batches for %s: %w", chrt.Name(), err)
	}

	// Deploy each subchart batch in order
	for batchIdx, batch := range batches {
		for _, subchartName := range batch {
			subManifests := grouped[subchartName]
			if len(subManifests) == 0 {
				continue
			}

			// Find the subchart chart object for recursive nested sequencing
			subChart := findSubchart(chrt, subchartName)
			if subChart == nil {
				// Subchart not found in chart object (may have been disabled or aliased differently)
				// Fall back to flat resource-group deployment for these manifests
				slog.Warn("subchart not found in chart dependencies; deploying without subchart sequencing",
					"subchart", subchartName,
					"batch", batchIdx,
				)
				if err := s.deployResourceGroupBatches(ctx, subManifests); err != nil {
					return fmt.Errorf("deploying subchart %s resources: %w", subchartName, err)
				}
				continue
			}

			// Recursively deploy the subchart (handles its own nested subcharts and resource-groups)
			if err := s.deployChartLevel(ctx, subChart, subManifests); err != nil {
				return fmt.Errorf("deploying subchart %s: %w", subchartName, err)
			}
		}
	}

	// Deploy parent chart's own resources (after all subchart batches complete)
	parentManifests := grouped[""]
	if len(parentManifests) > 0 {
		if err := s.deployResourceGroupBatches(ctx, parentManifests); err != nil {
			return fmt.Errorf("deploying %s own resources: %w", chrt.Name(), err)
		}
	}

	return nil
}

func batchHasCustomReadiness(manifests []releaseutil.Manifest) bool {
	for _, manifest := range manifests {
		if manifest.Head == nil || manifest.Head.Metadata == nil {
			continue
		}
		annotations := manifest.Head.Metadata.Annotations
		if annotations == nil {
			continue
		}
		if annotations[kube.AnnotationReadinessSuccess] != "" || annotations[kube.AnnotationReadinessFailure] != "" {
			return true
		}
	}
	return false
}

// deployResourceGroupBatches deploys manifests for a single chart level using
// resource-group annotation DAG ordering. Resources without group annotations
// (or with invalid ones) are deployed last.
func (s *sequencedDeployment) deployResourceGroupBatches(ctx context.Context, manifests []releaseutil.Manifest) error {
	if len(manifests) == 0 {
		return nil
	}

	result, _, err := releaseutil.ParseResourceGroups(manifests)
	if err != nil {
		return fmt.Errorf("parsing resource-group annotations: %w", err)
	}

	// If there are sequenced groups, build their DAG and deploy in order
	if len(result.Groups) > 0 {
		dag, err := releaseutil.BuildResourceGroupDAG(result)
		if err != nil {
			return fmt.Errorf("building resource-group DAG: %w", err)
		}

		batches, err := dag.GetBatches()
		if err != nil {
			return fmt.Errorf("getting resource-group batches: %w", err)
		}

		for _, groupBatch := range batches {
			var batchManifests []releaseutil.Manifest
			for _, groupName := range groupBatch {
				batchManifests = append(batchManifests, result.Groups[groupName]...)
			}
			if err := s.createAndWait(ctx, batchManifests); err != nil {
				return err
			}
		}
	}

	// Deploy unsequenced resources last
	if len(result.Unsequenced) > 0 {
		if err := s.createAndWait(ctx, result.Unsequenced); err != nil {
			return err
		}
	}

	return nil
}

// helmSequencingAnnotations lists annotation keys used internally by Helm for
// resource sequencing. These are stripped from resources before applying to
// Kubernetes because some (e.g. helm.sh/depends-on/resource-groups) contain
// multiple slashes which is invalid per the K8s annotation key format.
var helmSequencingAnnotations = []string{
	releaseutil.AnnotationDependsOnResourceGroups,
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
		for _, key := range helmSequencingAnnotations {
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

// createAndWait creates (or updates, in upgrade mode) a set of manifest resources
// and waits for them to be ready. It respects both the per-batch readiness timeout
// and the overall operation deadline.
func (s *sequencedDeployment) createAndWait(ctx context.Context, manifests []releaseutil.Manifest) error {
	if s.upgradeMode {
		return s.updateAndWait(ctx, manifests)
	}

	if len(manifests) == 0 {
		return nil
	}

	yaml := buildManifestYAML(manifests)
	resources, err := s.cfg.KubeClient.Build(bytes.NewBufferString(yaml), !s.disableOpenAPI)
	if err != nil {
		return fmt.Errorf("building resource batch: %w", err)
	}
	if len(resources) == 0 {
		return nil
	}

	if err := resources.Visit(setMetadataVisitor(s.releaseName, s.releaseNamespace, true)); err != nil {
		return fmt.Errorf("setting metadata for resource batch: %w", err)
	}

	if err := stripSequencingAnnotations(resources); err != nil {
		return fmt.Errorf("stripping sequencing annotations: %w", err)
	}

	result, err := s.cfg.KubeClient.Create(resources, kube.ClientCreateOptionServerSideApply(s.serverSideApply, false))
	if err != nil {
		return fmt.Errorf("creating resource batch: %w", err)
	}
	s.createdResources = append(s.createdResources, result.Created...)

	return s.waitForResources(resources, manifests)
}

// updateAndWait applies an upgrade batch using KubeClient.Update() and waits for readiness.
// It matches current (old) resources by objectKey to compute the per-batch diff.
func (s *sequencedDeployment) updateAndWait(_ context.Context, manifests []releaseutil.Manifest) error {
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

	// Find the subset of current (old) resources that are represented in this batch.
	// Update() will handle creates (target resources not in matchingCurrent) and
	// updates (resources in both). Deletions are handled separately after all batches.
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

	result, err := s.cfg.KubeClient.Update(
		matchingCurrent,
		target,
		kube.ClientUpdateOptionForceReplace(s.forceReplace),
		kube.ClientUpdateOptionServerSideApply(s.serverSideApply, s.forceConflicts),
		kube.ClientUpdateOptionUpgradeClientSideFieldManager(s.upgradeCSAFieldManager),
	)
	if err != nil {
		return fmt.Errorf("updating resource batch: %w", err)
	}
	s.createdResources = append(s.createdResources, result.Created...)

	return s.waitForResources(target, manifests)
}

// waitForResources waits for the given resources to become ready,
// applying the per-batch and overall deadline constraints.
func (s *sequencedDeployment) waitForResources(resources kube.ResourceList, manifests []releaseutil.Manifest) error {
	// Determine effective wait timeout: min(readinessTimeout, remaining time to overall deadline)
	waitTimeout := s.readinessTimeout
	if !s.deadline.IsZero() {
		remaining := time.Until(s.deadline)
		if remaining <= 0 {
			return fmt.Errorf("overall timeout exceeded before waiting for resource batch")
		}
		if waitTimeout <= 0 || remaining < waitTimeout {
			waitTimeout = remaining
		}
	}
	if waitTimeout <= 0 {
		waitTimeout = time.Minute // safe default
	}

	var err error
	var waiter kube.Waiter
	waitOptions := append([]kube.WaitOption(nil), s.waitOptions...)
	if batchHasCustomReadiness(manifests) {
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

// findSubchart finds the subchart chart object within chrt's direct dependencies by name or alias.
// It uses the parent chart's Metadata.Dependencies to resolve aliases, since the alias is stored
// on the Dependency struct in Chart.yaml, not on the subchart's own Metadata.
func findSubchart(chrt *chartv2.Chart, nameOrAlias string) *chartv2.Chart {
	// Build a map from subchart chart name → alias from the parent's dependency declarations.
	aliasMap := make(map[string]string) // chart name → effective name (alias or name)
	if chrt.Metadata != nil {
		for _, dep := range chrt.Metadata.Dependencies {
			effective := dep.Name
			if dep.Alias != "" {
				effective = dep.Alias
			}
			aliasMap[dep.Name] = effective
		}
	}

	for _, dep := range chrt.Dependencies() {
		effective := dep.Name()
		if alias, ok := aliasMap[dep.Name()]; ok {
			effective = alias
		}
		if effective == nameOrAlias || dep.Name() == nameOrAlias {
			return dep
		}
	}
	return nil
}
