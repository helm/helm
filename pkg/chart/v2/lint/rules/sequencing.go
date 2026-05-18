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
	"fmt"
	"path"
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
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

// Sequencing runs lint rules for HIP-0025 sequencing annotations.
func Sequencing(linter *support.Linter, namespace string, values map[string]any) {
	c, err := loader.LoadDir(linter.ChartDir)
	if err != nil {
		// Chart load errors are already reported by other lint rules.
		return
	}

	// ProcessDependencies must run before validateSubchartSequencing:
	// it prunes disabled subcharts from c.Dependencies() and applies
	// alias renames, which BuildSubchartDAG relies on.
	if err := chartutil.ProcessDependencies(c, values); err != nil {
		return
	}

	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateSubchartSequencing(c))
	validateRenderedSequencingAnnotations(linter, c, namespace, values)
}

func validateSubchartSequencing(c *chart.Chart) error {
	if c.Metadata == nil || len(c.Metadata.Dependencies) == 0 {
		return nil
	}

	dag, err := chartutil.BuildSubchartDAG(c)
	if err != nil {
		return err
	}
	if _, err := dag.GetBatches(); err != nil {
		return fmt.Errorf("subchart circular dependency detected: %w", err)
	}

	return nil
}

func validateRenderedSequencingAnnotations(linter *support.Linter, c *chart.Chart, namespace string, values map[string]any) {
	if err := chartutil.ProcessDependencies(c, values); err != nil {
		return
	}

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

	renderedContentMap, err := renderEngine.Render(c, valuesToRender)
	if err != nil {
		// Template rendering errors are already reported by the Templates lint rule.
		return
	}

	manifestsByChart := make(map[string][]releaseutil.Manifest)
	for templatePath, content := range renderedContentMap {
		if strings.TrimSpace(content) == "" {
			continue
		}

		chartPath := renderedTemplateChartPath(templatePath)
		for _, manifest := range parseRenderedManifests(content) {
			validateReadinessAnnotations(linter, templatePath, manifest)
			manifestsByChart[chartPath] = append(manifestsByChart[chartPath], manifest)
		}
	}

	for chartPath, manifests := range manifestsByChart {
		validateResourceGroupAnnotations(linter, chartPath, manifests)
	}
}

func renderedTemplateChartPath(templatePath string) string {
	if chartPath, _, ok := strings.Cut(templatePath, "/templates/"); ok {
		return chartPath
	}

	return path.Dir(templatePath)
}

func parseRenderedManifests(content string) []releaseutil.Manifest {
	rawManifests := releaseutil.SplitManifests(content)
	manifests := make([]releaseutil.Manifest, 0, len(rawManifests))

	for manifestName, raw := range rawManifests {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		var head releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(raw), &head); err != nil {
			continue
		}

		if head.Metadata != nil && head.Metadata.Name != "" {
			manifestName = head.Metadata.Name
		}

		manifests = append(manifests, releaseutil.Manifest{
			Name:    manifestName,
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
	hasSuccess := strings.TrimSpace(annotations[kube.AnnotationReadinessSuccess]) != ""
	hasFailure := strings.TrimSpace(annotations[kube.AnnotationReadinessFailure]) != ""
	if hasSuccess == hasFailure {
		return
	}

	linter.RunLinterRule(support.ErrorSev, templatePath, fmt.Errorf(
		"resource %q has only one of %q / %q annotations; both must be present or absent together",
		resourceDisplayName(manifest),
		kube.AnnotationReadinessSuccess,
		kube.AnnotationReadinessFailure,
	))
}

func validateResourceGroupAnnotations(linter *support.Linter, chartPath string, manifests []releaseutil.Manifest) {
	result, warnings, err := releaseutil.ParseResourceGroups(manifests)
	// HIP-0025: lint must fail on orphan resource-group dependencies and
	// malformed annotation JSON. Runtime falls back to the unsequenced batch
	// for graceful recovery, but the chart author should fix these at lint time.
	for _, warning := range warnings {
		linter.RunLinterRule(support.ErrorSev, chartPath, fmt.Errorf("%s", warning))
	}
	if err != nil {
		linter.RunLinterRule(support.ErrorSev, chartPath, err)
		return
	}

	dag, err := releaseutil.BuildResourceGroupDAG(result)
	if err != nil {
		linter.RunLinterRule(support.ErrorSev, chartPath, err)
		return
	}

	if _, err := dag.GetBatches(); err != nil {
		linter.RunLinterRule(support.ErrorSev, chartPath, fmt.Errorf("resource-group circular dependency detected: %w", err))
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
