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

package version

import (
	"errors"
	"runtime/debug"
	"slices"
)

// K8sIOClientGoModVersion reports the version of k8s.io/client-go this binary
// was built against, read from the module build info.
//
// client-go is included in the build graph by whatever links a Kubernetes
// client (for the helm CLI, that is cmd/helm). When a consumer links Helm's
// chart libraries WITHOUT a client-go dependency, build info will not list it
// and this returns an error; callers must tolerate that (see version.Get and
// chart/common capabilities).
func K8sIOClientGoModVersion() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("failed to read build info")
	}

	idx := slices.IndexFunc(info.Deps, func(m *debug.Module) bool {
		return m.Path == "k8s.io/client-go"
	})

	if idx == -1 {
		return "", errors.New("k8s.io/client-go not found in build info")
	}

	m := info.Deps[idx]

	return m.Version, nil
}
