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

package fake

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"helm.sh/helm/v4/pkg/kube"
)

// CapturingKubeClient is a fake KubeClient that captures internal flags such as the
// deletion propagation policy. It extends FailingKubeClient to provide dummy resources.
type CapturingKubeClient struct {
	FailingKubeClient
	// CapturedDeletionPropagation stores the DeletionPropagation value passed to Delete
	CapturedDeletionPropagation *string
}

// Delete captures the deletion propagation policy and returns success
func (c *CapturingKubeClient) Delete(resources kube.ResourceList, policy metav1.DeletionPropagation) (*kube.Result, []error) {
	if c.CapturedDeletionPropagation != nil {
		*c.CapturedDeletionPropagation = string(policy)
	}
	return &kube.Result{Deleted: resources}, nil
}
