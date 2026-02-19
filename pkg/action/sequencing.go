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
)

// GroupManifestsByDirectSubchart groups manifests by the direct subchart they belong to.
// The parent chart's own manifests (templates directly under `<chartName>/templates/`) are
// returned under the empty string key "".
// Subcharts are keyed by their immediate name under `<chartName>/charts/<subchart>/`.
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
		if !strings.HasPrefix(m.Name, chartsPrefix) {
			// Parent chart manifest
			result[""] = append(result[""], m)
			continue
		}
		// Extract the direct subchart name (first segment after "<chartName>/charts/")
		rest := m.Name[len(chartsPrefix):]
		// rest is like "subchart1/templates/deploy.yaml" or "subchart1/charts/nested/..."
		idx := strings.Index(rest, "/")
		if idx < 0 {
			// Unlikely: a file directly under charts/ with no subdirectory
			result[""] = append(result[""], m)
			continue
		}
		subchartName := rest[:idx]
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
	for _, m := range manifests {
		buf.WriteString(m.Content)
		buf.WriteString("\n")
	}
	return buf.String()
}

// sequencedDeployment performs ordered installation of chart resources.
// It handles the two-level DAG: first subchart ordering, then resource-group
// ordering within each chart level.
type sequencedDeployment struct {
	cfg              *Configuration
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

// deployResourceGroupBatches deploys manifests for a single chart level using
// resource-group annotation DAG ordering. Resources without group annotations
// (or with invalid ones) are deployed last.
func (s *sequencedDeployment) deployResourceGroupBatches(ctx context.Context, manifests []releaseutil.Manifest) error {
	if len(manifests) == 0 {
		return nil
	}

	result, warnings := releaseutil.ParseResourceGroups(manifests)
	for _, w := range warnings {
		slog.Warn("resource-group annotation warning", "warning", w)
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

// createAndWait creates a set of manifest resources and waits for them to be ready.
// It respects both the per-batch readiness timeout and the overall operation deadline.
func (s *sequencedDeployment) createAndWait(ctx context.Context, manifests []releaseutil.Manifest) error {
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

	_, err = s.cfg.KubeClient.Create(resources, kube.ClientCreateOptionServerSideApply(s.serverSideApply, false))
	if err != nil {
		return fmt.Errorf("creating resource batch: %w", err)
	}

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

	var waiter kube.Waiter
	if c, ok := s.cfg.KubeClient.(kube.InterfaceWaitOptions); ok {
		waiter, err = c.GetWaiterWithOptions(s.waitStrategy, s.waitOptions...)
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
func findSubchart(chrt *chartv2.Chart, nameOrAlias string) *chartv2.Chart {
	for _, dep := range chrt.Dependencies() {
		alias := dep.Metadata.Annotations["alias"]
		if alias == "" {
			alias = dep.Name()
		}
		if alias == nameOrAlias || dep.Name() == nameOrAlias {
			return dep
		}
	}
	return nil
}
