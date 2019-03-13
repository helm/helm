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

import "time"

// TestSuite comprises of the last run of the pre-defined test suite of a release version
type TestSuite struct {
	// StartedAt indicates the date/time this test suite was kicked off
	StartedAt time.Time `json:"started_at,omitempty"`
	// CompletedAt indicates the date/time this test suite was completed
	CompletedAt time.Time `json:"completed_at,omitempty"`
	// Results are the results of each segment of the test
	Results []*TestRun `json:"results,omitempty"`
}
