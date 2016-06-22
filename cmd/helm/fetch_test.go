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
	"fmt"

	"testing"
)

type testCase struct {
	in          string
	expectedErr error
	expectedOut string
}

var repos = map[string]string{
	"local":     "http://localhost:8879/charts",
	"someother": "http://storage.googleapis.com/mycharts",
}

var testCases = []testCase{
	{"bad", fmt.Errorf("Invalid chart url format: bad"), ""},
	{"http://", fmt.Errorf("Invalid chart url format: http://"), ""},
	{"http://example.com", fmt.Errorf("Invalid chart url format: http://example.com"), ""},
	{"http://example.com/foo/bar", nil, "http://example.com/foo/bar"},
	{"local/nginx-2.0.0.tgz", nil, "http://localhost:8879/charts/nginx-2.0.0.tgz"},
	{"nonexistentrepo/nginx-2.0.0.tgz", fmt.Errorf("No such repo: nonexistentrepo"), ""},
}

func testRunner(t *testing.T, tc testCase) {
	u, err := mapRepoArg(tc.in, repos)
	if (tc.expectedErr == nil && err != nil) ||
		(tc.expectedErr != nil && err == nil) ||
		(tc.expectedErr != nil && err != nil && tc.expectedErr.Error() != err.Error()) {
		t.Errorf("Expected mapRepoArg to fail with input %s %v but got %v", tc.in, tc.expectedErr, err)
	}

	if (u == nil && len(tc.expectedOut) != 0) ||
		(u != nil && len(tc.expectedOut) == 0) ||
		(u != nil && tc.expectedOut != u.String()) {
		t.Errorf("Expected %s to map to fetch url %v but got %v", tc.in, tc.expectedOut, u)
	}

}

func TestMappings(t *testing.T) {
	for _, tc := range testCases {
		testRunner(t, tc)
	}
}
