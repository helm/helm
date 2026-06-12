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

package rules // import "helm.sh/helm/v4/pkg/chart/v2/lint/rules"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/chart/common"
	commonutil "helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/engine"
	"helm.sh/helm/v4/pkg/kube"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/release/v1/sequence"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

// Sequencing runs lint rules for HIP-0025 sequencing annotations.
func Sequencing(linter *support.Linter, namespace string, values map[string]any) {
	c, err := loader.LoadDir(linter.ChartDir)
	if err != nil {
		return // chart load errors are reported by other lint rules
	}
	if err := chartutil.ProcessDependencies(c, values); err != nil {
		linter.RunLinterRule(support.ErrorSev, linter.ChartDir, err)
		return
	}

	// Render failures are reported by the Templates rule; manifests stays nil
	// so Build still validates the top-level subchart DAG (preserving the old
	// pre-render validateSubchartSequencing coverage for broken-template charts).
	manifests := collectRenderedManifests(linter, c, namespace, values)

	plan, err := sequence.Build(c, manifests)
	if err != nil {
		// Build's fatal classes are exactly what fails at install time:
		// subchart/resource-group cycles, unknown depends-on refs, malformed
		// helm.sh/depends-on/subcharts, multi-group assignment.
		linter.RunLinterRule(support.ErrorSev, linter.ChartDir, err)
		return
	}

	for _, w := range plan.Warnings {
		path := w.ChartPath
		if path == "" {
			path = linter.ChartDir
		}
		switch w.Kind {
		case sequence.WarningKindResourceGroupDemotion:
			// Runtime recovers by demoting; the chart author must fix these.
			linter.RunLinterRule(support.ErrorSev, path, errors.New(w.Message))
		case sequence.WarningKindPartialReadiness:
			// Already reported per-template (with better context) by
			// validateReadinessAnnotations during collection.
		default: // isolated group, undeclared/unresolved subchart
			linter.RunLinterRule(support.WarningSev, path, errors.New(w.Message))
		}
	}
}

func collectRenderedManifests(linter *support.Linter, c *chart.Chart, namespace string, values map[string]any) []releaseutil.Manifest {
	options := common.ReleaseOptions{
		Name:      "test-release",
		Namespace: namespace,
	}
	caps := common.DefaultCapabilities.Copy()

	coalescedValues, err := commonutil.CoalesceValues(c, values)
	if err != nil {
		return nil
	}

	valuesToRender, err := commonutil.ToRenderValues(c, coalescedValues, options, caps)
	if err != nil {
		return nil
	}

	var renderEngine engine.Engine
	renderEngine.LintMode = true

	renderedContentMap, err := renderEngine.RenderWithContext(context.Background(), c, valuesToRender)
	if err != nil {
		// Template rendering errors are already reported by the Templates lint rule.
		return nil
	}

	var manifests []releaseutil.Manifest
	for _, templatePath := range slices.Sorted(maps.Keys(renderedContentMap)) {
		content := renderedContentMap[templatePath]
		if strings.TrimSpace(content) == "" {
			continue
		}

		for _, manifest := range parseRenderedManifests(templatePath, content) {
			// HIP-0025 explicitly excludes hooks from sequencing: at install
			// time SortManifests routes hook resources out before resource-group
			// parsing runs, so their sequencing annotations are ignored. Mirror
			// that here — otherwise lint reads a hook's leftover
			// resource-group/depends-on annotations into the DAG and can report
			// spurious cycles or orphan errors that never occur at runtime.
			if isHookManifest(manifest) {
				continue
			}
			validateReadinessAnnotations(linter, templatePath, manifest)
			manifests = append(manifests, manifest)
		}
	}

	return manifests
}

// isHookManifest reports whether a rendered manifest is a Helm hook. Hooks
// carry the helm.sh/hook annotation and are excluded from HIP-0025 sequencing
// (the install path separates them via SortManifests), so the sequencing lint
// rules must skip them too.
func isHookManifest(manifest releaseutil.Manifest) bool {
	if manifest.Head == nil || manifest.Head.Metadata == nil {
		return false
	}
	return strings.TrimSpace(manifest.Head.Metadata.Annotations[release.HookAnnotation]) != ""
}

func parseRenderedManifests(templatePath, content string) []releaseutil.Manifest {
	rawManifests := releaseutil.SplitManifests(content)
	manifests := make([]releaseutil.Manifest, 0, len(rawManifests))

	for _, manifestName := range slices.Sorted(maps.Keys(rawManifests)) {
		raw := rawManifests[manifestName]
		if strings.TrimSpace(raw) == "" {
			continue
		}

		var head releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(raw), &head); err != nil {
			continue
		}

		manifests = append(manifests, releaseutil.Manifest{
			Name:    templatePath,
			Content: raw,
			Head:    &head,
		})
	}

	return manifests
}

func validateReadinessAnnotations(linter *support.Linter, templatePath string, manifest releaseutil.Manifest) {
	if manifest.Head == nil || manifest.Head.Metadata == nil {
		return
	}

	annotations := manifest.Head.Metadata.Annotations
	successRaw := strings.TrimSpace(annotations[kube.AnnotationReadinessSuccess])
	failureRaw := strings.TrimSpace(annotations[kube.AnnotationReadinessFailure])
	hasSuccess := successRaw != ""
	hasFailure := failureRaw != ""

	if hasSuccess != hasFailure {
		linter.RunLinterRule(support.ErrorSev, templatePath, fmt.Errorf(
			"resource %q has only one of %q / %q annotations; both must be present or absent together",
			resourceDisplayName(manifest),
			kube.AnnotationReadinessSuccess,
			kube.AnnotationReadinessFailure,
		))
		return
	}

	// Both present (or both absent). When present, the JSONPath expressions
	// must still be well-formed — the presence-symmetry check alone let
	// malformed expressions through at lint time.
	for key, raw := range map[string]string{
		kube.AnnotationReadinessSuccess: successRaw,
		kube.AnnotationReadinessFailure: failureRaw,
	} {
		if err := kube.ValidateReadinessExpressions(raw); err != nil {
			linter.RunLinterRule(support.ErrorSev, templatePath, fmt.Errorf(
				"resource %q has malformed %q annotation: %w",
				resourceDisplayName(manifest), key, err,
			))
		}
	}
}

func resourceDisplayName(manifest releaseutil.Manifest) string {
	if manifest.Head == nil || manifest.Head.Metadata == nil {
		return manifest.Name
	}

	if manifest.Head.Kind == "" {
		return manifest.Head.Metadata.Name
	}

	return fmt.Sprintf("%s/%s", manifest.Head.Kind, manifest.Head.Metadata.Name)
}
