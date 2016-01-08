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

type SemVer struct {
	Major uint
	Minor uint
	Patch uint
}

func NewSemVer(version string) (*SemVer, error) {
	result := &SemVer{}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) > 3 {
		return nil, fmt.Errorf("invalid semantic version: %s", version)
	}

	major, err := strconv.ParseUint(parts[0], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("invalid semantic version: %s", version)
	}

	result.Major = uint(major)
	if len(parts) < 3 {
		if len(parts) < 2 {
			if len(parts) < 1 {
				return nil, fmt.Errorf("invalid semantic version: %s", version)
			}
		} else {
			minor, err := strconv.ParseUint(parts[1], 10, 0)
			if err != nil {
				return nil, fmt.Errorf("invalid semantic version: %s", version)
			}

			result.Minor = uint(minor)
		}
	} else {
		patch, err := strconv.ParseUint(parts[2], 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid semantic version: %s", version)
		}

		result.Patch = uint(patch)
	}

	return result, nil
}

func (s *SemVer) String() string {
	result := strconv.Itoa(int(s.Major))
	if s.Minor != 0 || s.Patch != 0 {
		result = result + "." + strconv.Itoa(int(s.Minor))
	}

	if s.Patch != 0 {
		result = result + "." + strconv.Itoa(int(s.Patch))
	}

	return result
}
