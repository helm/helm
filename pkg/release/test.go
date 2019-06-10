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

package release

// Test defines a test object.
type Test struct {
	Name string `json:"name,omitempty"`
	// Kind is the Kubernetes kind.
	Kind string `json:"kind,omitempty"`
	// Path is the chart-relative path to the template.
	Path string `json:"path,omitempty"`
	// ExpectSuccess indicates whether test should be successful or fail
	ExpectSuccess bool `json:"expect_success,omitempty"`
	// Manifest is the manifest contents.
	Manifest string `json:"manifest,omitempty"`
}
