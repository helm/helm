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

package common

// DefaultVersionSet is the default set of Kubernetes API versions.
//
// This static snapshot avoids initializing the Kubernetes runtime scheme in
// binaries that only need chart capabilities. TestDefaultVersionSetMatchesScheme
// verifies that it remains in sync with the Kubernetes dependencies used by
// Helm.
var DefaultVersionSet = VersionSet{
	"v1",
	"admissionregistration.k8s.io/v1",
	"admissionregistration.k8s.io/v1alpha1",
	"admissionregistration.k8s.io/v1beta1",
	"internal.apiserver.k8s.io/v1alpha1",
	"apps/v1",
	"apps/v1beta1",
	"apps/v1beta2",
	"authentication.k8s.io/v1",
	"authentication.k8s.io/v1alpha1",
	"authentication.k8s.io/v1beta1",
	"authorization.k8s.io/v1",
	"authorization.k8s.io/v1beta1",
	"autoscaling/v1",
	"autoscaling/v2",
	"batch/v1",
	"batch/v1beta1",
	"certificates.k8s.io/v1",
	"certificates.k8s.io/v1beta1",
	"certificates.k8s.io/v1alpha1",
	"coordination.k8s.io/v1alpha2",
	"coordination.k8s.io/v1beta1",
	"coordination.k8s.io/v1",
	"discovery.k8s.io/v1",
	"discovery.k8s.io/v1beta1",
	"events.k8s.io/v1",
	"events.k8s.io/v1beta1",
	"extensions/v1beta1",
	"flowcontrol.apiserver.k8s.io/v1",
	"flowcontrol.apiserver.k8s.io/v1beta1",
	"flowcontrol.apiserver.k8s.io/v1beta2",
	"flowcontrol.apiserver.k8s.io/v1beta3",
	"networking.k8s.io/v1",
	"networking.k8s.io/v1beta1",
	"node.k8s.io/v1",
	"node.k8s.io/v1alpha1",
	"node.k8s.io/v1beta1",
	"policy/v1",
	"policy/v1beta1",
	"rbac.authorization.k8s.io/v1",
	"rbac.authorization.k8s.io/v1beta1",
	"rbac.authorization.k8s.io/v1alpha1",
	"resource.k8s.io/v1",
	"resource.k8s.io/v1beta2",
	"resource.k8s.io/v1beta1",
	"resource.k8s.io/v1alpha3",
	"scheduling.k8s.io/v1alpha2",
	"scheduling.k8s.io/v1beta1",
	"scheduling.k8s.io/v1",
	"storage.k8s.io/v1beta1",
	"storage.k8s.io/v1",
	"storage.k8s.io/v1alpha1",
	"storagemigration.k8s.io/v1beta1",
	"apiextensions.k8s.io/v1beta1",
	"apiextensions.k8s.io/v1",
}
