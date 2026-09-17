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

package gitupdate

import (
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuth(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		options     AuthOptions
		expected    any
		expectError string
	}{
		{
			name:     "none",
			url:      "https://example.com/repo.git",
			options:  AuthOptions{Type: AuthNone},
			expected: nil,
		},
		{
			name: "basic",
			url:  "https://example.com/repo.git",
			options: AuthOptions{
				Type:     AuthBasic,
				Username: "alice",
				Password: "secret",
			},
			expected: &githttp.BasicAuth{},
		},
		{
			name: "token defaults username",
			url:  "https://example.com/repo.git",
			options: AuthOptions{
				Type:  AuthToken,
				Token: "secret",
			},
			expected: &githttp.BasicAuth{},
		},
		{
			name: "bearer",
			url:  "https://example.com/repo.git",
			options: AuthOptions{
				Type:  AuthBearer,
				Token: "secret",
			},
			expected: &githttp.TokenAuth{},
		},
		{
			name: "auto selects token",
			url:  "https://example.com/repo.git",
			options: AuthOptions{
				Type:  AuthAuto,
				Token: "secret",
			},
			expected: &githttp.BasicAuth{},
		},
		{
			name:        "basic requires username",
			url:         "https://example.com/repo.git",
			options:     AuthOptions{Type: AuthBasic, Password: "secret"},
			expectError: "requires --username",
		},
		{
			name:        "unknown mode",
			url:         "https://example.com/repo.git",
			options:     AuthOptions{Type: "magic"},
			expectError: "unknown authentication mode",
		},
		{
			name:        "HTTP authentication rejects SSH URL",
			url:         "git@example.com:group/repo.git",
			options:     AuthOptions{Type: AuthToken, Token: "secret"},
			expectError: "requires an HTTP(S) repository URL",
		},
		{
			name:        "SSH authentication rejects HTTP URL",
			url:         "https://example.com/repo.git",
			options:     AuthOptions{Type: AuthSSHAgent},
			expectError: "requires an SSH repository URL",
		},
		{
			name:        "host-key option requires explicit SSH authentication",
			url:         "git@example.com:group/repo.git",
			options:     AuthOptions{Type: AuthNone, InsecureHostKey: true},
			expectError: "SSH host-key options require",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := BuildAuth(test.url, test.options)
			if test.expectError != "" {
				require.ErrorContains(t, err, test.expectError)
				return
			}
			require.NoError(t, err)
			if test.expected == nil {
				assert.Nil(t, actual)
				return
			}
			assert.IsType(t, test.expected, actual)
			if token, ok := actual.(*githttp.BasicAuth); ok && test.name == "token defaults username" {
				assert.Equal(t, "git", token.Username)
			}
		})
	}
}
