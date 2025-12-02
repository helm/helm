package action

import (
	"fmt"
	"helm.sh/helm/v3/pkg/release"
	"os"
	"sort"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/releaseutil"
)

func GetInstallSequence(ch *chart.Chart, manifests []releaseutil.Manifest, resources kube.ResourceList, hooks []*release.Hook) ([]InstallItem, error) {

	if len(ch.Metadata.Install) == 0 {
		return []InstallItem{
			{
				ChartName: ch.Name(),
				WaitFor:   "",
				Resources: resources,
			},
		}, nil
	}

	// 构建 manifest content -> chart name 的映射
	// 用资源的 Kind+Name 作为 key 来匹配
	type resourceKey struct {
		kind string
		name string
	}
	manifestChartMap := make(map[resourceKey]string)

	for _, m := range manifests {

		if m.Head == nil || m.Head.Metadata == nil {
			fmt.Fprintf(os.Stdout, "DEBUG: invalid manifest: %s in %s", m.Content, m.Name)
			continue
		}

		chartName := extractChartNameFromPath(m.Name)
		// 从 manifest content 中解析出 kind 和 name
		kind := m.Head.Kind
		name := m.Head.Metadata.Name
		if kind != "" && name != "" {
			manifestChartMap[resourceKey{kind: kind, name: name}] = chartName
		}
	}

	// 构建 waitFor 映射
	waitForMap := make(map[string]string)
	for _, inst := range ch.Metadata.Install {
		waitForMap[inst.Name] = inst.WaitFor
	}

	// 按 chart 分组
	groupMap := make(map[string]*InstallItem)
	var order []string

	for _, inst := range ch.Metadata.Install {
		groupMap[inst.Name] = &InstallItem{
			ChartName: inst.Name,
			WaitFor:   inst.WaitFor,
			Resources: kube.ResourceList{},
		}
		order = append(order, inst.Name)
	}

	// 遍历 resources，通过 Kind+Name 找到对应的 chart
	for _, r := range resources {
		kind := r.Mapping.GroupVersionKind.Kind
		name := r.Name

		key := resourceKey{kind: kind, name: name}
		chartName, found := manifestChartMap[key]
		if !found {
			chartName = ch.Name() // fallback 到主 chart
		}

		if group, exists := groupMap[chartName]; exists {
			group.Resources = append(group.Resources, r)
		} else {
			groupMap[chartName] = &InstallItem{
				ChartName: chartName,
				WaitFor:   waitForMap[chartName],
				Resources: kube.ResourceList{r},
			}
			order = append(order, chartName)
		}
	}

	for _, h := range hooks {
		chartName := extractChartNameFromPath(h.Path)

		item, exists := groupMap[chartName]
		if !exists {
			continue
		}

		for _, event := range h.Events {
			switch event {
			case release.HookPreInstall:
				item.PreInstallHooks = append(item.PreInstallHooks, h)
			case release.HookPostInstall:
				item.PostInstallHooks = append(item.PostInstallHooks, h)
			}
		}
	}

	// 对每个 item 的 hooks 按 weight 排序
	for _, item := range groupMap {
		sort.Slice(item.PreInstallHooks, func(i, j int) bool {
			return item.PreInstallHooks[i].Weight < item.PreInstallHooks[j].Weight
		})
		sort.Slice(item.PostInstallHooks, func(i, j int) bool {
			return item.PostInstallHooks[i].Weight < item.PostInstallHooks[j].Weight
		})
	}

	result := make([]InstallItem, 0, len(order))
	for _, name := range order {
		if item := groupMap[name]; item != nil && len(item.Resources) > 0 {
			result = append(result, *item)
		}
	}

	return result, nil
}

// 从 manifest 路径中提取 chart name
func extractChartNameFromPath(filePath string) string {
	parts := strings.Split(filePath, "/")

	for i, part := range parts {
		if part == "charts" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	if len(parts) >= 1 {
		return parts[0]
	}

	return filePath
}

// 简单解析 manifest content 获取 kind 和 name
func parseKindAndName(content string) (kind string, name string) {
	lines := strings.Split(content, "\n")

	inMetadata := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 解析 kind
		if strings.HasPrefix(trimmed, "kind:") {
			kind = strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
		}

		// 进入 metadata 块
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}

		// 在 metadata 块中找 name
		if inMetadata && strings.HasPrefix(trimmed, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			// 移除可能的引号
			name = strings.Trim(name, "\"'")
			break
		}

		// 离开 metadata 块
		if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			inMetadata = false
		}
	}

	return kind, name
}
