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

package cli

import "testing"

func TestSettingsValidation(t *testing.T) {
	tests := []struct {
		name string

		// input
		settings Settings

		// expected
		expectedErrs []error
	}{
		{settings: *SettingsFromEnv(), expectedErrs: []error{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errsEqual = func(es1, es2 []error) bool {
				return true
			}

			errs := tt.settings.Validate()
			if !errsEqual(errs, tt.expectedErrs) {
				t.Errorf("expected errors %v, got %v", tt.expectedErrs, errs)
			}
		})
	}
}
