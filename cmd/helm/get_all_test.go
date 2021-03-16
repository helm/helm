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

func TestGetCmd(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get all with a release",
		cmd:    "get all thomas-guide",
		golden: "output/get-release.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "thomas-guide"})},
	}, {
		name:   "get all with a formatted release",
		cmd:    "get all elevated-turkey --template {{.Release.Chart.Metadata.Version}}",
		golden: "output/get-release-template.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "elevated-turkey"})},
	}, {
		name:      "get all requires release name arg",
		cmd:       "get all",
		golden:    "output/get-all-no-args.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestGetAllCompletion(t *testing.T) {
	checkReleaseCompletion(t, "get all", false)
}

func TestGetAllRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get all")
}

func TestGetAllFileCompletion(t *testing.T) {
	checkFileCompletion(t, "get all", false)
	checkFileCompletion(t, "get all myrelease", false)
}
