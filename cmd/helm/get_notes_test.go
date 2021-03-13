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

func TestGetNotesCmd(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get notes of a deployed release",
		cmd:    "get notes the-limerick",
		golden: "output/get-notes.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "the-limerick"})},
	}, {
		name:      "get notes without args",
		cmd:       "get notes",
		golden:    "output/get-notes-no-args.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestGetNotesCompletion(t *testing.T) {
	checkReleaseCompletion(t, "get notes", false)
}

func TestGetNotesRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get notes")
}

func TestGetNotesFileCompletion(t *testing.T) {
	checkFileCompletion(t, "get notes", false)
	checkFileCompletion(t, "get notes myrelease", false)
}
