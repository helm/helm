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

package registry

import (
	"fmt"
	"strings"

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
				// Return the plus form: the pull path converts it back
				// to the underscore tag when fetching, and user-facing
				// uses (such as the pulled file name) show the chart
				// version, not the storage encoding.
				return strings.ReplaceAll(v, "_", "+"), nil
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
		// Tags are the original registry strings, so a tag stored under
		// the underscore convention from #10166 (1.1.17_meta) is not valid
		// semver as written. Parse its plus form for the constraint
		// check, but return the original string with only the
		// underscore converted: normalizing through semver would
		// rewrite loose tags like v1.2.0 to 1.2.0, which is not a
		// tag in the registry. The pull path converts the plus
		// form back to the underscore tag when fetching, and
		// returning the stored form would leak the storage
		// encoding into user-facing uses such as the pulled file name.
		test, err := semver.NewVersion(strings.ReplaceAll(v, "_", "+"))
		if err != nil {
			continue
		}
		if constraint.Check(test) {
			return strings.ReplaceAll(v, "_", "+"), nil
		}
	}

	return "", fmt.Errorf("could not locate a version matching provided version string %s", versionString)
}
