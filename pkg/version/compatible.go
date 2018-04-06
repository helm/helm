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
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
)

// IsCompatible tests if a client and server version are compatible.
func IsCompatible(client, server string) bool {
	if isUnreleased(client) || isUnreleased(server) {
		return true
	}
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

	return IsCompatibleRange(constraint, server)
}

// IsCompatibleRange compares a version to a constraint.
// It returns true if the version matches the constraint, and false in all other cases.
func IsCompatibleRange(constraint, ver string) bool {
	sv, err := semver.NewVersion(ver)
	if err != nil {
		return false
	}

	verSuffix := sv.Prerelease()
	if verSuffix != "" {
		if !IsConstraintPrerelease(constraint) {
			constraint = constraint + "-r0"
			trimmedVer := strings.TrimSuffix(sv.String(), "-"+verSuffix)
			sv, err = semver.NewVersion(trimmedVer)
			if err != nil {
				return false
			}
		}
	}

	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false
	}
	return c.Check(sv)
}

func IsConstraintPrerelease(constraint string) bool {
	base := constraint
	for _, c := range constraint {
		char := string(c)
		r := regexp.MustCompile(`^[0-9]+`).MatchString(char)
		if !r {
			base = strings.TrimPrefix(base, char)
		} else {
			break
		}
	}
	baseVer, err := semver.NewVersion(base)
	if err != nil {
		return false
	}
	if baseVer.Prerelease() == "" {
		return false
	}
	return true
}

func isUnreleased(v string) bool {
	return strings.HasSuffix(v, "unreleased")
}
