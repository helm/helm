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
	"strings"

	"helm.sh/helm/v4/pkg/kube"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func filterManifestsToKeep(manifests []releaseutil.Manifest) (keep, remaining []releaseutil.Manifest) {
	for _, m := range manifests {
		if m.Head.Metadata == nil || m.Head.Metadata.Annotations == nil || len(m.Head.Metadata.Annotations) == 0 {
			remaining = append(remaining, m)
			continue
		}

		resourcePolicyType, ok := m.Head.Metadata.Annotations[kube.ResourcePolicyAnno]
		if !ok {
			remaining = append(remaining, m)
			continue
		}

		if isKeepPolicy(resourcePolicyType) {
			keep = append(keep, m)
		}
	}
	return keep, remaining
}

// filterResourcesToKeep splits resources by the resource policy annotation on
// their objects. Resources whose annotations cannot be read are not kept.
func filterResourcesToKeep(resources kube.ResourceList) (keep, remaining kube.ResourceList) {
	for _, info := range resources {
		annotations, err := accessor.Annotations(info.Object)
		if err == nil && isKeepPolicy(annotations[kube.ResourcePolicyAnno]) {
			keep = append(keep, info)
			continue
		}
		remaining = append(remaining, info)
	}
	return keep, remaining
}

func isKeepPolicy(resourcePolicyType string) bool {
	return strings.ToLower(strings.TrimSpace(resourcePolicyType)) == kube.KeepPolicy
}
