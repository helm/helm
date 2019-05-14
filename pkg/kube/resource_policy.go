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

package kube

const (
	// ResourcePolicyAnno is the annotation name for a resource policy
	ResourcePolicyAnno = "helm.sh/resource-policy"

	// deletePolicy is the resource policy type for delete
	//
	// This resource policy type allows explicitly opting in to the default
	//   resource deletion behavior, for example when overriding a chart's
	//   default annotations. Any other value allows resources to skip being
	//   deleted during an uninstallRelease action.
	deletePolicy = "delete"
)

// ResourcePolicyIsKeep accepts a map of Kubernetes resource annotations and
//   returns true if the resource should be kept, otherwise false if it is safe
//   for Helm to delete.
func ResourcePolicyIsKeep(annotations map[string]string) bool {
	if annotations != nil {
		resourcePolicyType, ok := annotations[ResourcePolicyAnno]
		if ok && resourcePolicyType != deletePolicy {
			return true
		}
	}
	return false
}
