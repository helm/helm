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

package installer

import "testing"

func TestIsRemoteHTTPArchive(t *testing.T) {
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	if isRemoteHTTPArchive("/not/a/URL") {
		t.Errorf("Expected non-URL to return false")
	}

	// URLs with valid archive extensions are considered valid archives
	// even if the server is unreachable (optimization to avoid unnecessary HTTP requests)
	if !isRemoteHTTPArchive("https://127.0.0.1:123/fake/plugin-1.2.3.tgz") {
		t.Errorf("URL with .tgz extension should be considered a valid archive")
	}

	// Test with invalid extension and unreachable server
	if isRemoteHTTPArchive("https://127.0.0.1:123/fake/plugin-1.2.3.notanarchive") {
		t.Errorf("Bad URL without valid extension should not succeed")
	}

	if !isRemoteHTTPArchive(source) {
		t.Errorf("Expected %q to be a valid archive URL", source)
	}

	if isRemoteHTTPArchive(source + "-not-an-extension") {
		t.Error("Expected media type match to fail")
	}
}

func TestCheckVCSReference(t *testing.T) {
	testCases := []struct {
		name        string
		vcsURL      string
		expectedVCS string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Git SCP-like syntax",
			vcsURL:      "git@github.com:user/repo",
			expectedVCS: "git",
			expectError: false,
		},
		{
			name:        "Git URL with git username",
			vcsURL:      "https://git@github.com/user/repo.git",
			expectedVCS: "git",
			expectError: false,
		},
		{
			name:        "Mercurial URL with hg username",
			vcsURL:      "https://hg@bitbucket.org/user/repo",
			expectedVCS: "hg",
			expectError: false,
		},
		{
			name:        "Git URL with git scheme",
			vcsURL:      "git://github.com/user/repo.git",
			expectedVCS: "git",
			expectError: false,
		},
		{
			name:        "Git SSH URL",
			vcsURL:      "git+ssh://git@github.com/user/repo.git",
			expectedVCS: "git",
			expectError: false,
		},
		{
			name:        "Bazaar SSH URL",
			vcsURL:      "bzr+ssh://user@example.com/repo",
			expectedVCS: "bzr",
			expectError: false,
		},
		{
			name:        "SVN SSH URL",
			vcsURL:      "svn+ssh://user@example.com/repo",
			expectedVCS: "svn",
			expectError: false,
		},
		{
			name:        "Local file URL",
			vcsURL:      "file:///path/to/repo",
			expectedVCS: "file",
			expectError: false,
		},
		{
			name:        "Invalid URL",
			vcsURL:      "://invalid-url",
			expectError: true,
			errorMsg:    "parse plugin VCS url error",
		},
		{
			name:        "Empty host",
			vcsURL:      "http:///path/to/repo",
			expectError: true,
			errorMsg:    "invalid plugin VCS url, can't get correct host",
		},
		{
			name:        "Unknown scheme",
			vcsURL:      "unknown://github.com/user/repo",
			expectError: true,
			errorMsg:    "invalid plugin VCS url with unknown scheme",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vcsType, err := checkVCSReference(tc.vcsURL)
			if tc.expectError && err == nil {
				t.Error("expected error, but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectError && vcsType != tc.expectedVCS {
				t.Errorf("checkVCSReference() testCase is %v. but now vcsType = %s, error = %v", tc, vcsType, err)
			}
		})
	}
}
