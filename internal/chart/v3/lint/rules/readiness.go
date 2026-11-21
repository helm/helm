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

package rules

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	"helm.sh/helm/v4/internal/chart/v3/loader"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	release "helm.sh/helm/v4/internal/release/v2"
	releaseutil "helm.sh/helm/v4/internal/release/v2/util"
	"helm.sh/helm/v4/pkg/chart/common"
	commonutil "helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/engine"
	"helm.sh/helm/v4/pkg/kube"
)

// Readiness validates custom readiness annotations in rendered manifests.
func Readiness(linter *support.Linter, values map[string]any, namespace string) {
	c, err := loader.Load(linter.ChartDir)
	if err != nil {
		return // chart load errors are reported by other lint rules
	}
	if err := chartutil.ProcessDependencies(c, values); err != nil {
		linter.RunLinterRule(support.ErrorSev, linter.ChartDir, err)
		return
	}

	collectRenderedManifests(linter, c, namespace, values)
}

func collectRenderedManifests(linter *support.Linter, c *chart.Chart, namespace string, values map[string]any) {
	options := common.ReleaseOptions{
		Name:      "test-release",
		Namespace: namespace,
	}
	caps := common.DefaultCapabilities.Copy()

	coalescedValues, err := commonutil.CoalesceValues(c, values)
	if err != nil {
		return
	}

	valuesToRender, err := commonutil.ToRenderValues(c, coalescedValues, options, caps)
	if err != nil {
		return
	}

	var renderEngine engine.Engine
	renderEngine.LintMode = true

	renderedContentMap, err := renderEngine.RenderWithContext(context.Background(), c, valuesToRender)
	if err != nil {
		// Template rendering errors are already reported by the Templates rule.
		return
	}

	for _, templatePath := range slices.Sorted(maps.Keys(renderedContentMap)) {
		content := renderedContentMap[templatePath]
		if strings.TrimSpace(content) == "" {
			continue
		}

		for _, manifest := range parseRenderedManifests(templatePath, content) {
			if isHookManifest(manifest) {
				continue
			}
			validateReadinessAnnotations(linter, templatePath, manifest)
		}
	}
}

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

	successExprs, successErr := kube.ParseReadinessExpressions(successRaw)
	if successErr != nil {
		reportMalformedReadinessAnnotation(linter, templatePath, manifest, kube.AnnotationReadinessSuccess, successErr)
	}
	failureExprs, failureErr := kube.ParseReadinessExpressions(failureRaw)
	if failureErr != nil {
		reportMalformedReadinessAnnotation(linter, templatePath, manifest, kube.AnnotationReadinessFailure, failureErr)
	}

	hasSuccess := len(successExprs) > 0
	hasFailure := len(failureExprs) > 0
	if hasSuccess != hasFailure {
		linter.RunLinterRule(support.ErrorSev, templatePath, fmt.Errorf(
			"resource %q has only one of %q / %q annotations; both must be present or absent together",
			resourceDisplayName(manifest),
			kube.AnnotationReadinessSuccess,
			kube.AnnotationReadinessFailure,
		))
		return
	}

	for _, annotation := range []struct {
		key   string
		raw   string
		valid bool
	}{
		{kube.AnnotationReadinessSuccess, successRaw, successErr == nil && hasSuccess},
		{kube.AnnotationReadinessFailure, failureRaw, failureErr == nil && hasFailure},
	} {
		if !annotation.valid {
			continue
		}
		if err := kube.ValidateReadinessExpressions(annotation.raw); err != nil {
			reportMalformedReadinessAnnotation(linter, templatePath, manifest, annotation.key, err)
		}
	}
}

func reportMalformedReadinessAnnotation(linter *support.Linter, templatePath string, manifest releaseutil.Manifest, key string, err error) {
	linter.RunLinterRule(support.ErrorSev, templatePath, fmt.Errorf(
		"resource %q has malformed %q annotation: %w",
		resourceDisplayName(manifest), key, err,
	))
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
