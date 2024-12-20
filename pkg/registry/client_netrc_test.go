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

package registry

import (
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/netrc"
)

func TestClientNetrcAuth(t *testing.T) {
	// Create a temporary .netrc file
	content := `machine example.com
	login testuser
	password testpass`

	tmpDir := t.TempDir()
	netrcPath := filepath.Join(tmpDir, ".netrc")
	if err := os.WriteFile(netrcPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Set NETRC env var to point to our test file
	oldNetrc := os.Getenv("NETRC")
	defer os.Setenv("NETRC", oldNetrc)
	os.Setenv("NETRC", netrcPath)

	// Create a new client
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	// Test that credentials from .netrc are used
	creds, err := netrc.GetCredentials("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil {
		t.Fatal("Expected credentials from .netrc, got nil")
	}
	if creds.Login != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", creds.Login)
	}
	if creds.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", creds.Password)
	}

	// Test that explicit credentials override .netrc
	client, err = NewClient(
		ClientOptBasicAuth("explicituser", "explicitpass"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if client.username != "explicituser" {
		t.Errorf("Expected username 'explicituser', got '%s'", client.username)
	}
	if client.password != "explicitpass" {
		t.Errorf("Expected password 'explicitpass', got '%s'", client.password)
	}
}
