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
	tests := []releaseCase{{
		name:          "basic test",
		cmd:           "test example-release",
		testRunStatus: map[string]release.TestRunStatus{"PASSED: green lights everywhere": release.TestRunSuccess},
		golden:        "output/test.txt",
	}, {
		name:          "test failure",
		cmd:           "test example-fail",
		testRunStatus: map[string]release.TestRunStatus{"FAILURE: red lights everywhere": release.TestRunFailure},
		wantError:     true,
		golden:        "output/test-failure.txt",
	}, {
		name:          "test unknown",
		cmd:           "test example-unknown",
		testRunStatus: map[string]release.TestRunStatus{"UNKNOWN: yellow lights everywhere": release.TestRunUnknown},
		golden:        "output/test-unknown.txt",
	}, {
		name:          "test error",
		cmd:           "test example-error",
		testRunStatus: map[string]release.TestRunStatus{"ERROR: yellow lights everywhere": release.TestRunFailure},
		wantError:     true,
		golden:        "output/test-error.txt",
	}, {
		name:          "test running",
		cmd:           "test example-running",
		testRunStatus: map[string]release.TestRunStatus{"RUNNING: things are happpeningggg": release.TestRunRunning},
		golden:        "output/test-running.txt",
	}, {
		name: "multiple tests example",
		cmd:  "test example-suite",
		testRunStatus: map[string]release.TestRunStatus{
			"RUNNING: things are happpeningggg":           release.TestRunRunning,
			"PASSED: party time":                          release.TestRunSuccess,
			"RUNNING: things are happening again":         release.TestRunRunning,
			"FAILURE: good thing u checked :)":            release.TestRunFailure,
			"RUNNING: things are happpeningggg yet again": release.TestRunRunning,
			"PASSED: feel free to party again":            release.TestRunSuccess},
		wantError: true,
	}}
	testReleaseCmd(t, tests)
}
