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
	"fmt"
	"strings"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

// SubchartManifests groups rendered manifest content by subchart name.
// The parent chart's own resources are keyed by the chart name itself.
type SubchartManifests map[string]string

// SplitManifestsBySubchart splits a rendered manifest string into per-subchart
// manifest strings based on the # Source: comments that Helm inserts during rendering.
//
// Each manifest document is attributed to a subchart by parsing the Source path:
//   - "parentchart/templates/foo.yaml" → parent chart
//   - "parentchart/charts/subchart/templates/bar.yaml" → "subchart"
//   - "parentchart/charts/sub1/charts/sub2/templates/baz.yaml" → "sub1" (immediate child)
//
// The chartName parameter is the root chart name used to identify parent resources.
func SplitManifestsBySubchart(manifest string, chartName string) SubchartManifests {
	result := make(SubchartManifests)

	// Split on YAML document separator
	docs := strings.Split(manifest, "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		owner := chartName // default: parent chart owns this resource
		// Look for # Source: comment to determine subchart
		for line := range strings.SplitSeq(doc, "\n") {
			trimmed := strings.TrimSpace(line)
			if sourcePath, ok := strings.CutPrefix(trimmed, "# Source: "); ok {
				owner = subchartFromSourcePath(sourcePath, chartName)
				break
			}
		}

		if existing, ok := result[owner]; ok {
			result[owner] = existing + "\n---\n" + doc
		} else {
			result[owner] = doc
		}
	}

	return result
}

// subchartFromSourcePath extracts the immediate subchart name from a Source path.
// Path format: "chartname/charts/subchartname/templates/file.yaml"
// or: "chartname/templates/file.yaml" (parent chart)
func subchartFromSourcePath(sourcePath, chartName string) string {
	parts := strings.Split(sourcePath, "/")
	// Find "charts" segment — the next segment is the subchart name
	for i, part := range parts {
		if part == "charts" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// No "charts" segment → belongs to parent chart
	return chartName
}

// BuildInstallBatches constructs ordered batches of subchart names for installation.
// Returns batches where each batch can be installed in parallel, and batches
// are processed sequentially. The parent chart name is always in the last batch.
//
// If the chart has no sequencing metadata, returns nil (all resources deployed at once).
func BuildInstallBatches(chrt *chart.Chart) ([][]string, error) {
	if chrt.Metadata == nil {
		return nil, nil
	}

	// Check if any sequencing declarations exist
	hasAnnotation := false
	if chrt.Metadata.Annotations != nil {
		_, hasAnnotation = chrt.Metadata.Annotations[chartutil.AnnotationDependsOnSubcharts]
	}

	hasDependsOn := false
	for _, dep := range chrt.Metadata.Dependencies {
		if len(dep.DependsOn) > 0 {
			hasDependsOn = true
			break
		}
	}

	if !hasAnnotation && !hasDependsOn {
		return nil, nil
	}

	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return nil, fmt.Errorf("failed to build subchart dependency graph: %w", err)
	}

	batches, err := dag.Batches()
	if err != nil {
		return nil, fmt.Errorf("failed to compute installation order: %w", err)
	}

	return batches, nil
}
