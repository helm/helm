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
	"testing"
)

func TestParseInvalidVersionFails(t *testing.T) {
	for _, test := range []string{
		".",
		"..",
		"...",
		"1.2.3.4",
		"notAUnit",
		"1.notAUint",
		"1.1.notAUint",
		"-1",
		"1.-1",
		"1.1.-1",
		"1,1",
		"1.1,1",
	} {
		_, err := ParseSemVer(test)
		if err == nil {
			t.Errorf("Invalid version parsed successfully: %s\n", test)
		}
	}
}

func TestParseValidVersionSucceeds(t *testing.T) {
	for _, test := range []struct {
		String  string
		Version SemVer
	}{
		{"", SemVer{0, 0, 0}},
		{"0", SemVer{0, 0, 0}},
		{"0.0", SemVer{0, 0, 0}},
		{"0.0.0", SemVer{0, 0, 0}},
		{"1", SemVer{1, 0, 0}},
		{"1.0", SemVer{1, 0, 0}},
		{"1.0.0", SemVer{1, 0, 0}},
		{"1.1", SemVer{1, 1, 0}},
		{"1.1.0", SemVer{1, 1, 0}},
		{"1.1.1", SemVer{1, 1, 1}},
	} {
		result, err := ParseSemVer(test.String)
		if err != nil {
			t.Errorf("Valid version %s did not parse successfully\n", test.String)
		}

		if result.Major != test.Version.Major ||
			result.Minor != test.Version.Minor ||
			result.Patch != test.Version.Patch {
			t.Errorf("Valid version %s did not parse correctly: %s\n", test.String, test.Version)
		}
	}
}

func TestConvertSemVerToStringSucceeds(t *testing.T) {
	for _, test := range []struct {
		String  string
		Version SemVer
	}{
		{"0", SemVer{0, 0, 0}},
		{"0.1", SemVer{0, 1, 0}},
		{"0.0.1", SemVer{0, 0, 1}},
		{"1", SemVer{1, 0, 0}},
		{"1.1", SemVer{1, 1, 0}},
		{"1.1.1", SemVer{1, 1, 1}},
	} {
		result := test.Version.String()
		if result != test.String {
			t.Errorf("Valid version %s did not format correctly: %s\n", test.Version, test.String)
		}
	}
}
