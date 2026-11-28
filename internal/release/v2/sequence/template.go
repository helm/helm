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

package sequence

import (
	"strings"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

// TemplateManifest returns the plan's manifests in deployment order, in the
// format of a stored release manifest. As HIP-0025 asks for `helm template`,
// a "## START resource-group: <chart path> <group>" line opens the first
// document of each resource group and a matching "## END resource-group:"
// line closes its last document.
func (p *Plan) TemplateManifest() string {
	if p == nil {
		return ""
	}

	var sb strings.Builder
	for _, batch := range p.Batches {
		if batch.Kind != BatchKindGroups {
			for _, manifest := range batch.Manifests {
				writeTemplateManifest(&sb, manifest, "", "")
			}
			continue
		}
		chartPath := DisplayPath(batch.ChartPath)
		for _, group := range batch.Groups {
			for i, manifest := range group.Manifests {
				start := ""
				if i == 0 {
					start = group.Name
				}
				writeTemplateManifest(&sb, manifest, chartPath, start)
			}
			if len(group.Manifests) > 0 {
				writeGroupDelimiter(&sb, "END", chartPath, group.Name)
			}
		}
	}
	return sb.String()
}

// writeTemplateManifest writes one manifest document. When startGroup is set,
// the document opens with the START delimiter of that group.
func writeTemplateManifest(sb *strings.Builder, manifest releaseutil.Manifest, chartPath, startGroup string) {
	sb.WriteString("---\n")
	if startGroup != "" {
		writeGroupDelimiter(sb, "START", chartPath, startGroup)
	}
	sb.WriteString("# Source: ")
	sb.WriteString(manifest.Name)
	sb.WriteString("\n")
	sb.WriteString(manifest.Content)
	sb.WriteString("\n")
}

func writeGroupDelimiter(sb *strings.Builder, marker, chartPath, group string) {
	sb.WriteString("## ")
	sb.WriteString(marker)
	sb.WriteString(" resource-group: ")
	sb.WriteString(chartPath)
	sb.WriteString(" ")
	sb.WriteString(group)
	sb.WriteString("\n")
}
