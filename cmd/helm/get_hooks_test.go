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

	"helm.sh/helm/v3/pkg/release"
)

func TestGetHooks(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get hooks with release",
		cmd:    "get hooks aeneas",
		golden: "output/get-hooks.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "aeneas"})},
	}, {
		name:      "get hooks without args",
		cmd:       "get hooks",
		golden:    "output/get-hooks-no-args.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestGetHooksCompletion(t *testing.T) {
	checkReleaseCompletion(t, "get hooks", false)
}

func TestGetHooksRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get hooks")
}

func TestGetHooksFileCompletion(t *testing.T) {
	checkFileCompletion(t, "get hooks", false)
	checkFileCompletion(t, "get hooks myrelease", false)
}
