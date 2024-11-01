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
	"time"

	"helm.sh/helm/v3/pkg/kube"
)

// updateWithTimeoutOrFallback is a compatibility function to fallback for Helm 3 clients (implementors of kube.Interface only)
// this function can be inlined in Helm 4, when there is no fallback necessary anymore.
func updateWithTimeoutOrFallback(kubeClient kube.Interface, original, target kube.ResourceList, force bool, timeout time.Duration) (*kube.Result, error) {
	if kubeClient, ok := kubeClient.(kube.UpdateWithTimeout); ok {
		return kubeClient.UpdateWithTimeout(original, target, force, timeout)
	}
	return kubeClient.Update(original, target, force)
}
