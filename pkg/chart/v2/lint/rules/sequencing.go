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

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/common"
	commonutil "helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/engine"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

const (
	annotationReadinessSuccess = "helm.sh/readiness-success"
	annotationReadinessFailure = "helm.sh/readiness-failure"
)

// Sequencing runs lint rules for HIP-0025 sequencing annotations.
//
// It checks for:
//   - Circular dependencies in subchart ordering (error)
//   - Partial readiness annotations (only one of readiness-success/readiness-failure) (error)
//   - Resource-group depends-on referencing a non-existent group (warning)
func Sequencing(linter *support.Linter, namespace string, values map[string]interface{}) {
	c, err := loader.LoadDir(linter.ChartDir)
	if err != nil {
		// Chart load errors are already reported by other lint rules (Chartfile, Dependencies).
		// Silently skip sequencing checks rather than producing duplicate messages.
		return
	}

	// Check subchart circular dependencies.
	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateSubchartSequencing(c))

	// Check resource annotation issues via rendered templates.
	validateRenderedSequencingAnnotations(linter, c, namespace, values)
}

// validateSubchartSequencing checks for circular dependencies in subchart ordering.
//
// All dependencies are treated as enabled since conditions and tags cannot be
// evaluated at lint time (they depend on runtime values).
func validateSubchartSequencing(c *chart.Chart) error {
	if c.Metadata == nil || len(c.Metadata.Dependencies) == 0 {
		return nil
	}

	// In lint context, conditions/tags cannot be evaluated. Treat all deps as
	// enabled to catch cycles that would manifest with any values configuration.
	for _, dep := range c.Metadata.Dependencies {
		dep.Enabled = true
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

// validateRenderedSequencingAnnotations renders templates and checks sequencing
// annotation correctness:
//   - Only one of readiness-success/readiness-failure present → error
//   - depends-on/resource-groups referencing non-existent group → warning
func validateRenderedSequencingAnnotations(linter *support.Linter, c *chart.Chart, namespace string, values map[string]interface{}) {
	if err := chartutil.ProcessDependencies(c, values); err != nil {
		return
	}

	opts := common.ReleaseOptions{
		Name:      "test-release",
		Namespace: namespace,
	}
	caps := common.DefaultCapabilities.Copy()

	cvals, err := commonutil.CoalesceValues(c, values)
	if err != nil {
		return
	}

	valuesToRender, err := commonutil.ToRenderValues(c, cvals, opts, caps)
	if err != nil {
		return
	}

	var e engine.Engine
	e.LintMode = true
	renderedContentMap, err := e.Render(c, valuesToRender)
	if err != nil {
		// Template rendering errors are already caught by the Templates lint rule.
		return
	}

	// Collect all rendered YAML across templates.
	var allContent strings.Builder
	for _, t := range c.Templates {
		content := renderedContentMap[path.Join(c.Name(), t.Name)]
		if strings.TrimSpace(content) != "" {
			allContent.WriteString(content)
			allContent.WriteString("\n")
		}
	}

	// Parse rendered manifests into Manifest structs for annotation checking.
	rawManifests := releaseutil.SplitManifests(allContent.String())
	var manifests []releaseutil.Manifest
	for _, raw := range rawManifests {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var head releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(raw), &head); err != nil {
			continue
		}
		name := ""
		if head.Metadata != nil {
			name = head.Metadata.Name
		}
		manifests = append(manifests, releaseutil.Manifest{
			Name:    name,
			Content: raw,
			Head:    &head,
		})
	}

	// Check partial readiness annotations.
	for _, m := range manifests {
		if m.Head == nil || m.Head.Metadata == nil {
			continue
		}
		ann := m.Head.Metadata.Annotations
		_, hasSuccess := ann[annotationReadinessSuccess]
		_, hasFailure := ann[annotationReadinessFailure]
		if hasSuccess != hasFailure {
			linter.RunLinterRule(support.ErrorSev, linter.ChartDir,
				fmt.Errorf("resource %q has only one of %q / %q annotations; both must be present or absent together",
					m.Head.Metadata.Name, annotationReadinessSuccess, annotationReadinessFailure))
		}
	}

	// Check resource-group depends-on references via ParseResourceGroups.
	// Warnings indicate references to non-existent groups.
	_, warnings := releaseutil.ParseResourceGroups(manifests)
	for _, w := range warnings {
		linter.RunLinterRule(support.WarningSev, linter.ChartDir, fmt.Errorf("%s", w))
	}
}
