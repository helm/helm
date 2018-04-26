/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	"k8s.io/helm/pkg/hapi/release"
)

func TestReleaseTesting(t *testing.T) {
	tests := []releaseCase{
		{
			name:      "basic test",
			cmd:       "test example-release",
			responses: map[string]release.TestRunStatus{"PASSED: green lights everywhere": release.TestRun_SUCCESS},
			wantError: false,
		},
		{
			name:      "test failure",
			cmd:       "test example-fail",
			responses: map[string]release.TestRunStatus{"FAILURE: red lights everywhere": release.TestRun_FAILURE},
			wantError: true,
		},
		{
			name:      "test unknown",
			cmd:       "test example-unknown",
			responses: map[string]release.TestRunStatus{"UNKNOWN: yellow lights everywhere": release.TestRun_UNKNOWN},
			wantError: false,
		},
		{
			name:      "test error",
			cmd:       "test example-error",
			responses: map[string]release.TestRunStatus{"ERROR: yellow lights everywhere": release.TestRun_FAILURE},
			wantError: true,
		},
		{
			name:      "test running",
			cmd:       "test example-running",
			responses: map[string]release.TestRunStatus{"RUNNING: things are happpeningggg": release.TestRun_RUNNING},
			wantError: false,
		},
		{
			name: "multiple tests example",
			cmd:  "test example-suite",
			responses: map[string]release.TestRunStatus{
				"RUNNING: things are happpeningggg":           release.TestRun_RUNNING,
				"PASSED: party time":                          release.TestRun_SUCCESS,
				"RUNNING: things are happening again":         release.TestRun_RUNNING,
				"FAILURE: good thing u checked :)":            release.TestRun_FAILURE,
				"RUNNING: things are happpeningggg yet again": release.TestRun_RUNNING,
				"PASSED: feel free to party again":            release.TestRun_SUCCESS},
			wantError: true,
		},
	}
	testReleaseCmd(t, tests)
}
