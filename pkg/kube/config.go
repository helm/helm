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

package kube // import "helm.sh/helm/v4/pkg/kube"

import "k8s.io/cli-runtime/pkg/genericclioptions"

// GetConfig returns a Kubernetes client config.
//
// Deprecated
func GetConfig(kubeconfig, context, namespace string) *genericclioptions.ConfigFlags {
	cf := genericclioptions.NewConfigFlags(true)
	cf.Namespace = &namespace
	cf.Context = &context
	cf.KubeConfig = &kubeconfig
	return cf
}
