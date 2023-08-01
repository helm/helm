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

func TestGetMetadataCmd(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get metadata with a release",
		cmd:    "get metadata thomas-guide",
		golden: "output/get-metadata.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "thomas-guide"})},
	}, {
		name:      "get metadata requires release name arg",
		cmd:       "get metadata",
		golden:    "output/get-metadata-args.txt",
		rels:      []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "thomas-guide"})},
		wantError: true,
	}, {
		name:   "get metadata to json",
		cmd:    "get metadata thomas-guide --output json",
		golden: "output/get-metadata.json",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "thomas-guide"})},
	}, {
		name:   "get metadata to yaml",
		cmd:    "get metadata thomas-guide --output yaml",
		golden: "output/get-metadata.yaml",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "thomas-guide"})},
	}}
	runTestCmd(t, tests)
}

func TestGetMetadataCompletion(t *testing.T) {
	checkReleaseCompletion(t, "get metadata", false)
}

func TestGetMetadataRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get metadata")
}

func TestGetMetadataOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "get metadata")
}

func TestGetMetadataFileCompletion(t *testing.T) {
	checkFileCompletion(t, "get metadata", false)
	checkFileCompletion(t, "get metadata myrelease", false)
}
