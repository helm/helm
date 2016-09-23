/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package version // import "k8s.io/helm/pkg/version"

import (
	"fmt"

	"github.com/Masterminds/semver"
)

// IsCompatible tests if a client and server version are compatible.
func IsCompatible(client, server string) bool {
	cv, err := semver.NewVersion(client)
	if err != nil {
		return false
	}
	sv, err := semver.NewVersion(server)
	if err != nil {
		return false
	}

	constraint := fmt.Sprintf("^%d.%d.x", cv.Major(), cv.Minor())
	if cv.Prerelease() != "" || sv.Prerelease() != "" {
		constraint = cv.String()
	}

	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false
	}
	return c.Check(sv)
}
