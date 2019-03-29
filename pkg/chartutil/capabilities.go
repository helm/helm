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
	"encoding/json"
	"fmt"
	"runtime"
	"sort"

	"k8s.io/apimachinery/pkg/version"

	"k8s.io/client-go/kubernetes/scheme"
)

var (
	// DefaultVersionSet is the default version set, which includes only Core V1 ("v1").
	DefaultVersionSet = allKnownVersions()

	// DefaultKubeVersion is the default kubernetes version
	DefaultKubeVersion = &version.Info{
		Major:      "1",
		Minor:      "9",
		GitVersion: "v1.9.0",
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	// DefaultCapabilities is the default set of capabilities.
	DefaultCapabilities = &Capabilities{
		APIVersions: DefaultVersionSet,
		KubeVersion: DefaultKubeVersion,
	}
)

// Capabilities describes the capabilities of the Kubernetes cluster that Tiller is attached to.
type Capabilities struct {
	// List of all supported API versions
	APIVersions VersionSet
	// KubeVerison is the Kubernetes version
	KubeVersion *version.Info
}

// VersionSet is a set of Kubernetes API versions.
type VersionSet map[string]struct{}

// NewVersionSet creates a new version set from a list of strings.
func NewVersionSet(apiVersions ...string) VersionSet {
	vs := make(VersionSet)
	for _, v := range apiVersions {
		vs[v] = struct{}{}
	}
	return vs
}

// Has returns true if the version string is in the set.
//
//	vs.Has("apps/v1")
func (v VersionSet) Has(apiVersion string) bool {
	_, ok := v[apiVersion]
	return ok
}

func allKnownVersions() VersionSet {
	vs := make(VersionSet)
	for _, gv := range scheme.Scheme.PrioritizedVersionsAllGroups() {
		vs[gv.String()] = struct{}{}
	}
	return vs
}

// MarshalJSON implements the encoding/json.Marshaler interface.
func (v VersionSet) MarshalJSON() ([]byte, error) {
	out := make([]string, 0, len(v))
	for i := range v {
		out = append(out, i)
	}
	sort.Strings(out)
	return json.Marshal(out)
}

// UnmarshalJSON implements the encoding/json.Unmarshaler interface.
func (v *VersionSet) UnmarshalJSON(data []byte) error {
	var vs []string
	if err := json.Unmarshal(data, &vs); err != nil {
		return err
	}
	*v = NewVersionSet(vs...)
	return nil
}
