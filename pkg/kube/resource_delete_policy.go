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

package kube // import "helm.sh/helm/v3/pkg/kube"

import (
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ResourceDeletionPolicyAnno = "helm.sh/resource-deletion-policy"

// selectDeletionPolicy allows to select an override deleteion policy per resource,
// based on ResourceDeletionPolicyAnno.
func selectDeletionPolicy(policyAnnotation string, defualt v1.DeletionPropagation) v1.DeletionPropagation {
	switch policyAnnotation {
	case strings.ToLower(string(v1.DeletePropagationBackground)):
		return v1.DeletePropagationBackground
	case strings.ToLower(string(v1.DeletePropagationForeground)):
		return v1.DeletePropagationForeground
	case strings.ToLower(string(v1.DeletePropagationOrphan)):
		return v1.DeletePropagationOrphan
	default:
		return defualt
	}
}
