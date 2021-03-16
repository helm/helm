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

func TestGetManifest(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get manifest with release",
		cmd:    "get manifest juno",
		golden: "output/get-manifest.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "juno"})},
	}, {
		name:      "get manifest without args",
		cmd:       "get manifest",
		golden:    "output/get-manifest-no-args.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestGetManifestCompletion(t *testing.T) {
	checkReleaseCompletion(t, "get manifest", false)
}

func TestGetManifestRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get manifest")
}

func TestGetManifestFileCompletion(t *testing.T) {
	checkFileCompletion(t, "get manifest", false)
	checkFileCompletion(t, "get manifest myrelease", false)
}
