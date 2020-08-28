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

package rules // import "helm.sh/helm/v3/pkg/lint/rules"

import "fmt"

type deprecatedAPI struct {
	NewAPI       string
	DeprecatedIn string
	RemovedIn    string
}

// deprecatedAPIs lists APIs that are deprecated (key) with suggested alternatives (value).
var deprecatedAPIs = map[string]deprecatedAPI{
	"extensions/v1beta1 Deployment": {
		NewAPI:       "apps/v1 Deployment",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta1 Deployment": {
		NewAPI:       "apps/v1 Deployment",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta2 Deployment": {
		NewAPI:       "apps/v1 Deployment",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta1 StatefulSet": {
		NewAPI:       "apps/v1 StatefulSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta2 StatefulSet": {
		NewAPI:       "apps/v1 StatefulSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"extensions/v1beta1 DaemonSet": {
		NewAPI:       "apps/v1 DaemonSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta2 DaemonSet": {
		NewAPI:       "apps/v1 DaemonSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"extensions/v1beta1 ReplicaSet": {
		NewAPI:       "apps/v1 ReplicaSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta1 ReplicaSet": {
		NewAPI:       "apps/v1 ReplicaSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"apps/v1beta2 ReplicaSet": {
		NewAPI:       "apps/v1 ReplicaSet",
		DeprecatedIn: "v1.9",
		RemovedIn:    "v1.16",
	},
	"extensions/v1beta1 NetworkPolicy": {
		NewAPI:       "networking.k8s.io/v1 NetworkPolicy",
		DeprecatedIn: "v1.8",
		RemovedIn:    "v1.16",
	},
	"extensions/v1beta1 PodSecurityPolicy": {
		NewAPI:       "policy/v1beta1 PodSecurityPolicy",
		DeprecatedIn: "v1.10",
		RemovedIn:    "v1.16",
	},
	"apiextensions.k8s.io/v1beta1 CustomResourceDefinition": {
		NewAPI:       "apiextensions.k8s.io/v1 CustomResourceDefinition",
		DeprecatedIn: "v1.16",
		RemovedIn:    "v1.19",
	},
	"extensions/v1beta1 Ingress": {
		NewAPI:       "networking.k8s.io/v1beta1 Ingress",
		DeprecatedIn: "v1.14",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 ClusterRole": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRole",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 ClusterRoleList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRoleList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 ClusterRoleBinding": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRoleBinding",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 ClusterRoleBindingList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRoleBindingList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 Role": {
		NewAPI:       "rbac.authorization.k8s.io/v1 Role",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 RoleList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 RoleList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 RoleBinding": {
		NewAPI:       "rbac.authorization.k8s.io/v1 RoleBinding",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1alpha1 RoleBindingList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 RoleBindingList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta ClusterRole": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRole",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 ClusterRoleList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRoleList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 ClusterRoleBinding": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRoleBinding",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 ClusterRoleBindingList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 ClusterRoleBindingList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 Role": {
		NewAPI:       "rbac.authorization.k8s.io/v1 Role",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 RoleList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 RoleList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 RoleBinding": {
		NewAPI:       "rbac.authorization.k8s.io/v1 RoleBinding",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
	"rbac.authorization.k8s.io/v1beta1 RoleBindingList": {
		NewAPI:       "rbac.authorization.k8s.io/v1 RoleBindingList",
		DeprecatedIn: "v1.17",
		RemovedIn:    "v1.22",
	},
}

// deprecatedAPIError indicates than an API is deprecated in Kubernetes
type deprecatedAPIError struct {
	Deprecated  string
	Alternative string
	In          string
	Removed     string
}

func (e deprecatedAPIError) Error() string {
	msg := fmt.Sprintf("the kind %q is deprecated as of %q and removed in %q", e.Deprecated, e.In, e.Removed)
	if e.Alternative != "" {
		msg += fmt.Sprintf(" in favor of %q", e.Alternative)
	}
	return msg
}

func validateNoDeprecations(resource *K8sYamlStruct) error {
	gvk := fmt.Sprintf("%s %s", resource.APIVersion, resource.Kind)
	if dep, ok := deprecatedAPIs[gvk]; ok {
		return deprecatedAPIError{
			Deprecated:  gvk,
			Alternative: dep.NewAPI,
			In:          dep.DeprecatedIn,
			Removed:     dep.RemovedIn,
		}
	}
	return nil
}
