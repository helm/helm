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

package repo // import "helm.sh/helm/pkg/repo"

import (
	"github.com/containerd/containerd/reference"
)

// ParseNameTag converts a name and a version to a reference
func ParseNameTag(name, tag string) (reference.Spec, error) {
	s := name
	if tag != "" {
		s += ":" + tag
	}
	return reference.Parse(s)
}
