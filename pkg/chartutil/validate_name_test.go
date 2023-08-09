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

package chartutil

import "testing"

// TestValidateName is a regression test for ValidateName
//
// Kubernetes has strict naming conventions for resource names. This test represents
// those conventions.
//
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
//
// NOTE: At the time of this writing, the docs above say that names cannot begin with
// digits. However, `kubectl`'s regular expression explicit allows this, and
// Kubernetes (at least as of 1.18) also accepts resources whose names begin with digits.
func TestValidateReleaseName(t *testing.T) {
	names := map[string]bool{
		"":                          false,
		"foo":                       true,
		"foo.bar1234baz.seventyone": true,
		"FOO":                       false,
		"123baz":                    true,
		"foo.BAR.baz":               false,
		"one-two":                   true,
		"-two":                      false,
		"one_two":                   false,
		"a..b":                      false,
		"%^&#$%*@^*@&#^":            false,
		"example:com":               false,
		"example%%com":              false,
		"a1111111111111111111111111111111111111111111111111111111111z": false,
	}
	for input, expectPass := range names {
		if err := ValidateReleaseName(input); (err == nil) != expectPass {
			st := "fail"
			if expectPass {
				st = "succeed"
			}
			t.Errorf("Expected %q to %s", input, st)
		}
	}
}

func TestValidateMetadataName(t *testing.T) {
	names := map[string]bool{
		"":                          false,
		"foo":                       true,
		"foo.bar1234baz.seventyone": true,
		"FOO":                       false,
		"123baz":                    true,
		"foo.BAR.baz":               false,
		"one-two":                   true,
		"-two":                      false,
		"one_two":                   false,
		"a..b":                      false,
		"%^&#$%*@^*@&#^":            false,
		"example:com":               false,
		"example%%com":              false,
		"a1111111111111111111111111111111111111111111111111111111111z": true,
		"a1111111111111111111111111111111111111111111111111111111111z" +
			"a1111111111111111111111111111111111111111111111111111111111z" +
			"a1111111111111111111111111111111111111111111111111111111111z" +
			"a1111111111111111111111111111111111111111111111111111111111z" +
			"a1111111111111111111111111111111111111111111111111111111111z" +
			"a1111111111111111111111111111111111111111111111111111111111z": false,
	}
	for input, expectPass := range names {
		if err := ValidateMetadataName(input); (err == nil) != expectPass {
			st := "fail"
			if expectPass {
				st = "succeed"
			}
			t.Errorf("Expected %q to %s", input, st)
		}
	}
}
