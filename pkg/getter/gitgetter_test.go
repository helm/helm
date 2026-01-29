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

package getter

import (
	"testing"
)

func TestNewGitGetter(t *testing.T) {
	g, err := NewGitGetter()
	if err != nil {
		t.Skip("git or helm command not found in PATH, skipping test")
	}

	if _, ok := g.(*GitGetter); !ok {
		t.Fatal("Expected NewGitGetter to produce a *GitGetter")
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectRepo  string
		expectRef   string
		expectPath  string
		expectError bool
	}{
		{
			name:        "git:// with ref and path",
			url:         "git://github.com/user/repo@v1.0.0?path=charts/mychart",
			expectRepo:  "https://github.com/user/repo",
			expectRef:   "v1.0.0",
			expectPath:  "charts/mychart",
			expectError: false,
		},
		{
			name:        "git+https:// with ref",
			url:         "git+https://github.com/user/repo@main",
			expectRepo:  "https://github.com/user/repo",
			expectRef:   "main",
			expectPath:  "",
			expectError: false,
		},
		{
			name:        "git+https:// without ref",
			url:         "git+https://github.com/user/repo",
			expectRepo:  "https://github.com/user/repo",
			expectRef:   "HEAD",
			expectPath:  "",
			expectError: false,
		},
		{
			name:        "git+ssh:// with ref and path",
			url:         "git+ssh://git@github.com/user/repo@v2.0.0?path=charts/test",
			expectRepo:  "git@github.com:user/repo",
			expectRef:   "v2.0.0",
			expectPath:  "charts/test",
			expectError: false,
		},
		{
			name:        "git:// with ref query param",
			url:         "git://github.com/user/repo?ref=feature-branch&path=charts/app",
			expectRepo:  "https://github.com/user/repo",
			expectRef:   "feature-branch",
			expectPath:  "charts/app",
			expectError: false,
		},
		{
			name:        "git+http:// simple",
			url:         "git+http://gitlab.com/group/project@develop",
			expectRepo:  "http://gitlab.com/group/project",
			expectRef:   "develop",
			expectPath:  "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ref, path, err := parseGitURL(tt.url)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if repo != tt.expectRepo {
				t.Errorf("expected repo %q, got %q", tt.expectRepo, repo)
			}

			if ref != tt.expectRef {
				t.Errorf("expected ref %q, got %q", tt.expectRef, ref)
			}

			if path != tt.expectPath {
				t.Errorf("expected path %q, got %q", tt.expectPath, path)
			}
		})
	}
}

func TestGitGetterSchemes(t *testing.T) {
	// Test that Git getter is registered for the correct schemes
	schemes := []string{"git", "git+https", "git+http", "git+ssh"}

	providers := Getters()

	for _, scheme := range schemes {
		found := false
		for _, p := range providers {
			if p.Provides(scheme) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("scheme %q not registered with any provider", scheme)
		}
	}
}
