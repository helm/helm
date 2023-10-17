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

package main

import (
	"testing"
)

func TestDocsTypeFlagCompletion(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "completion for docs --type",
		cmd:    "__complete docs --type ''",
		golden: "output/docs-type-comp.txt",
	}, {
		name:   "completion for docs --type, no filter",
		cmd:    "__complete docs --type mar",
		golden: "output/docs-type-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestDocsFileCompletion(t *testing.T) {
	checkFileCompletion(t, "docs", false)
}
