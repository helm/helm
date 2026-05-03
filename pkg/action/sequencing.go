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
	"slices"
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
	return s.deployChartLevelAt(ctx, chrt, manifests, chrt.Name())
}

// deployChartLevelAt is the recursive worker. chartPath tracks the manifest
// path-prefix of the current chart level so nested subcharts route correctly:
// top-level "parent" → child "parent/charts/sub" → grandchild "parent/charts/sub/charts/nested".
func (s *sequencedDeployment) deployChartLevelAt(ctx context.Context, chrt *chartv2.Chart, manifests []releaseutil.Manifest, chartPath string) error {
	grouped := GroupManifestsByDirectSubchart(manifests, chartPath)

	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return fmt.Errorf("building subchart DAG for %s: %w", chrt.Name(), err)
	}

	batches, err := dag.GetBatches()
	if err != nil {
		return fmt.Errorf("getting subchart batches for %s: %w", chrt.Name(), err)
	}

	for batchIdx, batch := range batches {
		for _, subchartName := range batch {
			subManifests := grouped[subchartName]
			if len(subManifests) == 0 {
				continue
			}

			subChart := findSubchart(chrt, subchartName)
			if subChart == nil {
				s.cfg.Logger().Warn("subchart not found in chart dependencies; deploying without subchart sequencing",
					"subchart", subchartName,
					"batch", batchIdx,
				)
				if err := s.deployResourceGroupBatches(ctx, subManifests); err != nil {
					return fmt.Errorf("deploying subchart %s resources: %w", subchartName, err)
				}
				continue
			}

			subPath := chartPath + "/charts/" + subchartName
			if err := s.deployChartLevelAt(ctx, subChart, subManifests, subPath); err != nil {
				return fmt.Errorf("deploying subchart %s: %w", subchartName, err)
			}
		}
	}

	parentManifests := grouped[""]
	if len(parentManifests) > 0 {
		if err := s.deployResourceGroupBatches(ctx, parentManifests); err != nil {
			return fmt.Errorf("deploying %s own resources: %w", chrt.Name(), err)
		}
	}

	return nil
}

// warnIfPartialReadinessAnnotations logs a warning for each resource that has
// only one of the helm.sh/readiness-success / helm.sh/readiness-failure annotations.
// Both annotations must be present for custom readiness evaluation to work; a resource
// with only one will fall back to kstatus.
func warnIfPartialReadinessAnnotations(logger *slog.Logger, manifests []releaseutil.Manifest) {
	for _, m := range manifests {
		if m.Head == nil || m.Head.Metadata == nil {
			continue
		}
		ann := m.Head.Metadata.Annotations
		_, hasSuccess := ann[kube.AnnotationReadinessSuccess]
		_, hasFailure := ann[kube.AnnotationReadinessFailure]
		if hasSuccess != hasFailure {
			logger.Warn("resource has only one readiness annotation; both helm.sh/readiness-success and helm.sh/readiness-failure must be present for custom readiness evaluation; falling back to kstatus",
				"resource", m.Head.Metadata.Name,
			)
		}
	}
}

// warnIfIsolatedGroups logs a warning for each resource-group that has no
// connections to other groups (no depends-on edges and not depended on by any
// other group). Isolated groups are valid but may indicate a misconfiguration
// when multiple groups are present.
func warnIfIsolatedGroups(logger *slog.Logger, result releaseutil.ResourceGroupResult) {
	if len(result.Groups) <= 1 {
		return // A single group is not isolated in a meaningful sense
	}
	for groupName := range result.Groups {
		deps := result.GroupDeps[groupName]
		isDependedOn := false
		for _, otherDeps := range result.GroupDeps {
			if slices.Contains(otherDeps, groupName) {
				isDependedOn = true
				break
			}
		}
		if len(deps) == 0 && !isDependedOn {
			logger.Warn("resource-group is isolated with no connections to other groups; if sequencing is intended, add helm.sh/depends-on/resource-groups annotation to related groups",
				"group", groupName,
			)
		}
	}
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

	// Emit warnings for misconfigured annotations before deploying.
	warnIfPartialReadinessAnnotations(s.cfg.Logger(), manifests)

	result, warnings, err := releaseutil.ParseResourceGroups(manifests)
	if err != nil {
		return fmt.Errorf("parsing resource-group annotations: %w", err)
	}
	for _, w := range warnings {
		s.cfg.Logger().Warn("resource-group annotation warning", "warning", w)
	}

	// If there are sequenced groups, build their DAG and deploy in order
	if len(result.Groups) > 0 {
		// Warn about groups that have no connections to other groups.
		warnIfIsolatedGroups(s.cfg.Logger(), result)

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

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	result, err := s.cfg.KubeClient.Create(resources, kube.ClientCreateOptionServerSideApply(s.serverSideApply, false))
	if err != nil {
		return fmt.Errorf("creating resource batch: %w", err)
	}
	s.createdResources = append(s.createdResources, result.Created...)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return s.waitForResources(resources, manifests)
}

// updateAndWait applies an upgrade batch using KubeClient.Update() and waits for readiness.
// It matches current (old) resources by objectKey to compute the per-batch diff.
func (s *sequencedDeployment) updateAndWait(ctx context.Context, manifests []releaseutil.Manifest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

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
			return errors.New("overall timeout exceeded before waiting for resource batch")
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
