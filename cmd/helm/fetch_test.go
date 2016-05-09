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
