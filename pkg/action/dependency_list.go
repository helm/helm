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
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func DependencyStatus(chartpath string, dep *chart.Dependency, parent *chart.Chart) string {
	filename := fmt.Sprintf("%s-%s.tgz", dep.Name, "*")

	// If a chart is unpacked, this will check the unpacked chart's `charts/` directory for tarballs.
	// Technically, this is COMPLETELY unnecessary, and should be removed in Helm 4. It is here
	// to preserved backward compatibility. In Helm 2/3, there is a "difference" between
	// the tgz version (which outputs "ok" if it unpacks) and the loaded version (which outouts
	// "unpacked"). Early in Helm 2's history, this would have made a difference. But it no
	// longer does. However, since this code shipped with Helm 3, the output must remain stable
	// until Helm 4.
	switch archives, err := filepath.Glob(filepath.Join(chartpath, "charts", filename)); {
	case err != nil:
		return "bad pattern"
	case len(archives) > 1:
		return "too many matches"
	case len(archives) == 1:
		archive := archives[0]
		if _, err := os.Stat(archive); err == nil {
			c, err := loader.Load(archive)
			if err != nil {
				return "corrupt"
			}
			if c.Name() != dep.Name {
				return "misnamed"
			}

			if c.Metadata.Version != dep.Version {
				constraint, err := semver.NewConstraint(dep.Version)
				if err != nil {
					return "invalid version"
				}

				v, err := semver.NewVersion(c.Metadata.Version)
				if err != nil {
					return "invalid version"
				}

				if !constraint.Check(v) {
					return "wrong version"
				}
			}
			return "ok"
		}
	}
	// End unnecessary code.

	var depChart *chart.Chart
	for _, item := range parent.Dependencies() {
		if item.Name() == dep.Name {
			depChart = item
		}
	}

	if depChart == nil {
		return "missing"
	}

	if depChart.Metadata.Version != dep.Version {
		constraint, err := semver.NewConstraint(dep.Version)
		if err != nil {
			return "invalid version"
		}

		v, err := semver.NewVersion(depChart.Metadata.Version)
		if err != nil {
			return "invalid version"
		}

		if !constraint.Check(v) {
			return "wrong version"
		}
	}

	return "unpacked"
}
