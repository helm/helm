/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package tiller

import (
	"strings"
)

// resourcePolicyAnno is the annotation name for a resource policy
const resourcePolicyAnno = "helm.sh/resource-policy"

// keepPolicy is the resource policy type for keep
//
// This resource policy type allows resources to skip being deleted
//   during an uninstallRelease action.
const keepPolicy = "keep"

func filterManifestsToKeep(manifests []manifest) ([]manifest, []manifest) {
	remaining := []manifest{}
	keep := []manifest{}

	for _, m := range manifests {

		if m.head.Metadata == nil || m.head.Metadata.Annotations == nil || len(m.head.Metadata.Annotations) == 0 {
			remaining = append(remaining, m)
			continue
		}

		resourcePolicyType, ok := m.head.Metadata.Annotations[resourcePolicyAnno]
		if !ok {
			remaining = append(remaining, m)
			continue
		}

		resourcePolicyType = strings.ToLower(strings.TrimSpace(resourcePolicyType))
		if resourcePolicyType == keepPolicy {
			keep = append(keep, m)
		}

	}
	return keep, remaining
}

func summarizeKeptManifests(manifests []manifest) string {
	message := "These resources were kept due to the resource policy:\n"
	for _, m := range manifests {
		details := "[" + m.head.Kind + "] " + m.head.Metadata.Name + "\n"
		message = message + details
	}
	return message
}
