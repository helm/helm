/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"strconv"
	"strings"
)

// SemVer holds a semantic version as defined by semver.io.
type SemVer struct {
	Major uint
	Minor uint
	Patch uint
}

// ParseSemVer parses a semantic version string.
func ParseSemVer(version string) (SemVer, error) {
	var err error
	major, minor, patch := uint64(0), uint64(0), uint64(0)
	if len(version) > 0 {
		parts := strings.SplitN(version, ".", 3)
		if len(parts) > 3 {
			return SemVer{}, fmt.Errorf("invalid semantic version: %s", version)
		}

		if len(parts) < 1 {
			return SemVer{}, fmt.Errorf("invalid semantic version: %s", version)
		}

		if parts[0] != "0" {
			major, err = strconv.ParseUint(parts[0], 10, 0)
			if err != nil {
				return SemVer{}, fmt.Errorf("invalid semantic version: %s", version)
			}
		}

		if len(parts) > 1 {
			if parts[1] != "0" {
				minor, err = strconv.ParseUint(parts[1], 10, 0)
				if err != nil {
					return SemVer{}, fmt.Errorf("invalid semantic version: %s", version)
				}
			}

			if len(parts) > 2 {
				if parts[2] != "0" {
					patch, err = strconv.ParseUint(parts[2], 10, 0)
					if err != nil {
						return SemVer{}, fmt.Errorf("invalid semantic version: %s", version)
					}
				}
			}
		}
	}

	return SemVer{Major: uint(major), Minor: uint(minor), Patch: uint(patch)}, nil
}

// IsZero returns true if the semantic version is zero.
func (s SemVer) IsZero() bool {
	return s.Major == 0 && s.Minor == 0 && s.Patch == 0
}

// SemVer conforms to the Stringer interface.
func (s SemVer) String() string {
	result := strconv.Itoa(int(s.Major))
	if s.Minor != 0 || s.Patch != 0 {
		result = result + "." + strconv.Itoa(int(s.Minor))
	}

	if s.Patch != 0 {
		result = result + "." + strconv.Itoa(int(s.Patch))
	}

	return result
}
