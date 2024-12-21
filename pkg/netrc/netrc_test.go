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

package netrc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCredentials(t *testing.T) {
	// Create a temporary .netrc file
	content := `machine example.com
	login testuser
	password testpass
machine other.com
	login otheruser
	password otherpass`

	tmpDir := t.TempDir()
	netrcPath := filepath.Join(tmpDir, ".netrc")
	if err := os.WriteFile(netrcPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Set NETRC env var to point to our test file
	oldNetrc := os.Getenv("NETRC")
	defer os.Setenv("NETRC", oldNetrc)
	os.Setenv("NETRC", netrcPath)

	tests := []struct {
		name     string
		url      string
		wantUser string
		wantPass string
		wantErr  bool
	}{
		{
			name:     "basic URL",
			url:      "https://example.com/repo",
			wantUser: "testuser",
			wantPass: "testpass",
		},
		{
			name:     "URL with port",
			url:      "https://example.com:443/repo",
			wantUser: "testuser",
			wantPass: "testpass",
		},
		{
			name:     "other domain",
			url:      "https://other.com/repo",
			wantUser: "otheruser",
			wantPass: "otherpass",
		},
		{
			name:     "no match",
			url:      "https://nomatch.com/repo",
			wantUser: "",
			wantPass: "",
		},
		{
			name:    "invalid URL",
			url:     "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCredentials(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.wantUser == "" && tt.wantPass == "" {
				if got != nil {
					t.Errorf("GetCredentials() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("GetCredentials() = nil, want credentials")
			}
			if got.Login != tt.wantUser {
				t.Errorf("GetCredentials() username = %v, want %v", got.Login, tt.wantUser)
			}
			if got.Password != tt.wantPass {
				t.Errorf("GetCredentials() password = %v, want %v", got.Password, tt.wantPass)
			}
		})
	}
}

func TestParseNetrc(t *testing.T) {
	content := `# comment line
machine example.com
	login user1
	password pass1
machine other.com login user2 password pass2
machine "quoted.com"
	login "user 3"
	password "pass 3"
`
	p := newParser(content)
	machines, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}

	if len(machines) != 3 {
		t.Errorf("Expected 3 machines, got %d", len(machines))
	}

	expected := []machine{
		{Machine: "example.com", Login: "user1", Password: "pass1"},
		{Machine: "other.com", Login: "user2", Password: "pass2"},
		{Machine: "quoted.com", Login: "user 3", Password: "pass 3"},
	}

	for i, e := range expected {
		if machines[i] != e {
			t.Errorf("machine[%d] = %v, want %v", i, machines[i], e)
		}
	}
}
