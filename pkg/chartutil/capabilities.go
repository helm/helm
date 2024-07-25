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

	"github.com/Masterminds/semver/v3"
	"k8s.io/client-go/kubernetes/scheme"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	helmversion "helm.sh/helm/v3/internal/version"
)

var (
	// The Kubernetes version can be set by LDFLAGS. In order to do that the value
	// must be a string.
	k8sVersionMajor = "1"
	k8sVersionMinor = "20"

	// DefaultVersionSet is the default version set, which includes only Core V1 ("v1").
	DefaultVersionSet = allKnownVersions()

	// DefaultCapabilities is the default set of capabilities.
	DefaultCapabilities = &Capabilities{
		KubeVersion: KubeVersion{
			Version: fmt.Sprintf("v%s.%s.0", k8sVersionMajor, k8sVersionMinor),
			Major:   k8sVersionMajor,
			Minor:   k8sVersionMinor,
		},
		APIVersions: DefaultVersionSet,
		HelmVersion: helmversion.Get(),
	}
)

// Capabilities describes the capabilities of the Kubernetes cluster.
type Capabilities struct {
	// KubeVersion is the Kubernetes version.
	KubeVersion KubeVersion
	// APIVersions are supported Kubernetes API versions.
	APIVersions VersionSet
	// HelmVersion is the build information for this helm version
	HelmVersion helmversion.BuildInfo
}

func (capabilities *Capabilities) Copy() *Capabilities {
	return &Capabilities{
		KubeVersion: capabilities.KubeVersion,
		APIVersions: capabilities.APIVersions,
		HelmVersion: capabilities.HelmVersion,
	}
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

// ParseKubeVersion parses kubernetes version from string
func ParseKubeVersion(version string) (*KubeVersion, error) {
	sv, err := semver.NewVersion(version)
	if err != nil {
		return nil, err
	}
	return &KubeVersion{
		Version: "v" + sv.String(),
		Major:   strconv.FormatUint(sv.Major(), 10),
		Minor:   strconv.FormatUint(sv.Minor(), 10),
	}, nil
}

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
	apiextensionsv1beta1.AddToScheme(scheme.Scheme)
	apiextensionsv1.AddToScheme(scheme.Scheme)

	groups := scheme.Scheme.PrioritizedVersionsAllGroups()
	vs := make(VersionSet, 0, len(groups))
	for _, gv := range groups {
		vs = append(vs, gv.String())
	}
	return vs
}
