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

package releaseutil

import (
	"helm.sh/helm/v3/pkg/release"
	"sort"
	"strings"
)

type ChartSortOrder []string

func sortManifestsByChart(manifests []Manifest, ordering ChartSortOrder) []Manifest {
	sort.SliceStable(manifests, func(i, j int) bool {
		return lessByChart(manifests[i], manifests[j], manifests[i].Name, manifests[j].Name, ordering)
	})

	return manifests
}

func sortHooksByChart(hooks []*release.Hook, ordering ChartSortOrder) []*release.Hook {
	h := hooks
	sort.SliceStable(h, func(i, j int) bool {
		return lessByChart(h[i], h[j], h[i].Name, h[j].Name, ordering)
	})

	return h
}

func lessByChart(a interface{}, b interface{}, filePathA string, filePathB string, o ChartSortOrder) bool {
	ordering := make(map[string]int, len(o))
	for v, k := range o {
		ordering[k] = v
	}

	nameA := extractChartNameFromPath(filePathA)
	nameB := extractChartNameFromPath(filePathB)

	first, aok := ordering[nameA]
	second, bok := ordering[nameB]

	if !aok && !bok {
		// if both are unknown then sort alphabetically by kind, keep original order if same kind
		if nameA != nameB {
			return nameA < nameB
		}
		return first < second
	}
	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}
	// sort different kinds, keep original order if same priority
	return first < second
}

// 安全地从路径中提取 chart name
func extractChartNameFromPath(filePath string) string {
	parts := strings.Split(filePath, "/")

	// 查找 "charts" 关键字，取其后一个作为 chart name
	// 格式: dtc/charts/loki/templates/xxx.yaml -> loki
	for i, part := range parts {
		if part == "charts" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// 如果没有 "charts/"，说明是主 chart 的资源
	// 格式可能是: dtc/templates/xxx.yaml -> dtc (主 chart)
	// 或者其他格式
	if len(parts) > 0 {
		return parts[0] // 返回第一个部分作为主 chart name
	}

	return filePath // fallback
}
