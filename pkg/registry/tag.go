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

package registry // import "helm.sh/helm/v4/pkg/registry"

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

func GetTagMatchingVersionOrConstraint(tags []string, versionString string) (string, error) {
	var constraint *semver.Constraints
	if versionString == "" {
		// If the string is empty, set a wildcard constraint
		constraint, _ = semver.NewConstraint("*")
	} else {
		// when customer inputs a specific version, check whether there's an exact match first
		for _, v := range tags {
			if versionString == v {
				return v, nil
			}
		}

		// Otherwise set constraint to the string given
		var err error
		constraint, err = semver.NewConstraint(versionString)
		if err != nil {
			return "", err
		}
	}

	// Otherwise try to find the first available version matching the string,
	// in case it is a constraint
	for _, v := range tags {
		test, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		if constraint.Check(test) {
			return v, nil
		}
	}

	return "", fmt.Errorf("could not locate a version matching provided version string %s", versionString)
}
