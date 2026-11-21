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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v4/internal/release/v2/resourcegroup"
	"helm.sh/helm/v4/pkg/kube"
)

// stripSequencingAnnotations removes helm.sh/depends-on/resource-groups from
// the objects. Kubernetes rejects annotation keys with more than one '/', so
// the key must not reach the API server. The stored release manifest keeps it.
func stripSequencingAnnotations(resources kube.ResourceList) error {
	return resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		accessor, err := meta.Accessor(info.Object)
		if err != nil {
			return nil // objects without metadata carry no annotations
		}
		annotations := accessor.GetAnnotations()
		if _, ok := annotations[resourcegroup.AnnotationDependsOnResourceGroups]; !ok {
			return nil
		}
		delete(annotations, resourcegroup.AnnotationDependsOnResourceGroups)
		accessor.SetAnnotations(annotations)
		return nil
	})
}
