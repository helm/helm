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

package chartutil

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	helmversion "helm.sh/helm/v3/internal/version"
)

const (
	k8sVersionMajor = 1
	k8sVersionMinor = 20
)

// Capabilities describes the capabilities of the Kubernetes cluster.
type Capabilities struct {
	// KubeVersion is the Kubernetes version.
	KubeVersion KubeVersion
	// APIversions are supported Kubernetes API versions.
	APIVersions VersionSet
	// HelmVersion is the build information for this helm version
	HelmVersion helmversion.BuildInfo
}

// KubeVersion is the Kubernetes version.
type KubeVersion struct {
	Version string // Kubernetes version
	Major   string // Kubernetes major version
	Minor   string // Kubernetes minor version
}

// String implements fmt.Stringer
func (kv *KubeVersion) String() string { return kv.Version }

// GitVersion returns the Kubernetes version string.
//
// Deprecated: use KubeVersion.Version.
func (kv *KubeVersion) GitVersion() string { return kv.Version }

// VersionSet is a set of Kubernetes API versions.
type VersionSet []string

// Has returns true if the version string is in the set.
//
//	vs.Has("apps/v1")
func (v VersionSet) Has(apiVersion string) bool {
	for _, x := range v {
		if x == apiVersion {
			return true
		}
	}
	return false
}

func allKnownVersions() VersionSet {
	// We should register the built in extension APIs as well so CRDs are
	// supported in the default version set. This has caused problems with `helm
	// template` in the past, so let's be safe
	s := runtime.NewScheme()
	metav1.AddToGroupVersion(s, schema.GroupVersion{Version: "v1"})
	scheme.AddToScheme(s)

	addKnownTypesv1beta1 := func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(apiextensionsv1beta1.SchemeGroupVersion,
			&apiextensionsv1beta1.CustomResourceDefinition{},
			&apiextensionsv1beta1.CustomResourceDefinitionList{},
			&apiextensionsv1beta1.ConversionReview{},
		)
		metav1.AddToGroupVersion(scheme, apiextensionsv1beta1.SchemeGroupVersion)
		return nil
	}
	schemeBuilderv1beta1 := runtime.NewSchemeBuilder(addKnownTypesv1beta1, apiextensionsv1.RegisterDefaults)
	schemeBuilderv1beta1.AddToScheme(s)

	addKnownTypesv1 := func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(apiextensionsv1.SchemeGroupVersion,
			&apiextensionsv1.CustomResourceDefinition{},
			&apiextensionsv1.CustomResourceDefinitionList{},
			&apiextensionsv1.ConversionReview{},
		)
		metav1.AddToGroupVersion(scheme, apiextensionsv1.SchemeGroupVersion)
		return nil
	}
	schemeBuilderv1 := runtime.NewSchemeBuilder(addKnownTypesv1, apiextensionsv1.RegisterDefaults)
	schemeBuilderv1.AddToScheme(s)

	groups := s.PrioritizedVersionsAllGroups()
	vs := make(VersionSet, 0, len(groups))
	for _, gv := range groups {
		vs = append(vs, gv.String())
	}
	return vs
}

// DefaultCapabilities returns the default set of capabilities.
func DefaultCapabilities() *Capabilities {
	return &Capabilities{
		KubeVersion: KubeVersion{
			Version: fmt.Sprintf("v%d.%d.0", k8sVersionMajor, k8sVersionMinor),
			Major:   strconv.Itoa(k8sVersionMajor),
			Minor:   strconv.Itoa(k8sVersionMinor),
		},
		APIVersions: DefaultVersionSet(),
		HelmVersion: helmversion.Get(),
	}
}

// DefaultVersionSet is the default version set, which includes only Core V1 ("v1").
func DefaultVersionSet() VersionSet {
	return allKnownVersions()
}
