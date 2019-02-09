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

	"k8s.io/helm/pkg/release"
)

func TestGetValuesCmd(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get values with a release",
		cmd:    "get values thomas-guide",
		golden: "output/get-values.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "thomas-guide"})},
	}, {
		name:      "get values requires release name arg",
		cmd:       "get values",
		golden:    "output/get-values-args.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}
