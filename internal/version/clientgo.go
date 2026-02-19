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
	"fmt"
	"runtime/debug"
	"slices"

	_ "k8s.io/client-go/kubernetes" // Force k8s.io/client-go to be included in the build
)

func K8sIOClientGoModVersion() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("failed to read build info")
	}

	idx := slices.IndexFunc(info.Deps, func(m *debug.Module) bool {
		return m.Path == "k8s.io/client-go"
	})

	if idx == -1 {
		return "", fmt.Errorf("k8s.io/client-go not found in build info")
	}

	m := info.Deps[idx]

	return m.Version, nil
}
