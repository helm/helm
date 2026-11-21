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
	"errors"
	"maps"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	"helm.sh/helm/v4/internal/chart/v3/loader"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	release "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/internal/release/v2/sequence"
	releaseutil "helm.sh/helm/v4/internal/release/v2/util"
	"helm.sh/helm/v4/pkg/chart/common"
	commonutil "helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/engine"
)

// Sequencing runs lint rules for HIP-0025 subchart sequencing.
func Sequencing(linter *support.Linter, namespace string, values map[string]any) {
	chrt, err := loader.LoadDir(linter.ChartDir)
	if err != nil {
		return
	}
	if chrt.Metadata == nil || chrt.Metadata.APIVersion != chart.APIVersionV3 {
		return
	}
	if err := chartutil.ProcessDependencies(chrt, values); err != nil {
		linter.RunLinterRule(support.ErrorSev, linter.ChartDir, err)
		return
	}

	plan, err := sequence.Build(chrt, collectSequencingManifests(chrt, namespace, values))
	if err != nil {
		linter.RunLinterRule(support.ErrorSev, linter.ChartDir, err)
		return
	}

	for _, warning := range plan.Warnings {
		switch warning.Kind {
		case sequence.WarningKindUndeclaredSubchart, sequence.WarningKindUnresolvedSubchart:
			path := warning.ChartPath
			if path == "" {
				path = linter.ChartDir
			}
			linter.RunLinterRule(support.WarningSev, path, errors.New(warning.Message))
		}
	}
}

func collectSequencingManifests(chrt *chart.Chart, namespace string, values map[string]any) []releaseutil.Manifest {
	coalesced, err := commonutil.CoalesceValues(chrt, values)
	if err != nil {
		return nil
	}
	toRender, err := commonutil.ToRenderValues(chrt, coalesced, common.ReleaseOptions{
		Name:      "test-release",
		Namespace: namespace,
	}, common.DefaultCapabilities.Copy())
	if err != nil {
		return nil
	}

	renderer := engine.Engine{LintMode: true}
	rendered, err := renderer.RenderWithContext(context.Background(), chrt, toRender)
	if err != nil {
		return nil
	}

	var manifests []releaseutil.Manifest
	for _, templatePath := range slices.Sorted(maps.Keys(rendered)) {
		for _, manifest := range parseSequencingManifests(templatePath, rendered[templatePath]) {
			if !isHookManifest(manifest) {
				manifests = append(manifests, manifest)
			}
		}
	}
	return manifests
}

func parseSequencingManifests(templatePath, content string) []releaseutil.Manifest {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	rawManifests := releaseutil.SplitManifests(content)
	manifests := make([]releaseutil.Manifest, 0, len(rawManifests))
	for _, name := range slices.Sorted(maps.Keys(rawManifests)) {
		raw := rawManifests[name]
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var head releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(raw), &head); err != nil {
			continue
		}
		manifests = append(manifests, releaseutil.Manifest{Name: templatePath, Content: raw, Head: &head})
	}
	return manifests
}

func isHookManifest(manifest releaseutil.Manifest) bool {
	if manifest.Head == nil || manifest.Head.Metadata == nil {
		return false
	}
	return strings.TrimSpace(manifest.Head.Metadata.Annotations[release.HookAnnotation]) != ""
}
