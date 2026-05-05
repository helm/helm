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

package fileutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtomicWriteFile tests the happy path of AtomicWriteFile function.
// It verifies that the function correctly writes content to a file with the specified mode.
func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()

	testpath := filepath.Join(dir, "test")
	stringContent := "Test content"
	reader := bytes.NewReader([]byte(stringContent))
	mode := os.FileMode(0644)

	err := AtomicWriteFile(testpath, reader, mode)
	if err != nil {
		t.Errorf("AtomicWriteFile error: %s", err)
	}

	got, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatal(err)
	}

	if stringContent != string(got) {
		t.Fatalf("expected: %s, got: %s", stringContent, string(got))
	}

	gotinfo, err := os.Stat(testpath)
	if err != nil {
		t.Fatal(err)
	}

	if mode != gotinfo.Mode() {
		t.Fatalf("expected %s: to be the same mode as %s",
			mode, gotinfo.Mode())
	}
}

// TestAtomicWriteFile_CreateTempError tests the error path when os.CreateTemp fails
func TestAtomicWriteFile_CreateTempError(t *testing.T) {
	invalidPath := "/invalid/path/that/does/not/exist/testfile"

	reader := bytes.NewReader([]byte("test content"))
	mode := os.FileMode(0644)

	err := AtomicWriteFile(invalidPath, reader, mode)
	if err == nil {
		t.Error("Expected error when CreateTemp fails, but got nil")
	}
}

// TestAtomicWriteFile_EmptyContent tests with empty content
func TestAtomicWriteFile_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	testpath := filepath.Join(dir, "empty_helm")

	reader := bytes.NewReader([]byte(""))
	mode := os.FileMode(0644)

	err := AtomicWriteFile(testpath, reader, mode)
	if err != nil {
		t.Errorf("AtomicWriteFile error with empty content: %s", err)
	}

	got, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty content, got: %s", string(got))
	}
}

// TestAtomicWriteFile_LargeContent tests with large content
func TestAtomicWriteFile_LargeContent(t *testing.T) {
	dir := t.TempDir()
	testpath := filepath.Join(dir, "large_test")

	// Create a large content string
	largeContent := strings.Repeat("HELM", 1024*1024)
	reader := bytes.NewReader([]byte(largeContent))
	mode := os.FileMode(0644)

	err := AtomicWriteFile(testpath, reader, mode)
	if err != nil {
		t.Errorf("AtomicWriteFile error with large content: %s", err)
	}

	got, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatal(err)
	}

	if largeContent != string(got) {
		t.Fatalf("expected large content to match, got different length: %d vs %d", len(largeContent), len(got))
	}
}

// TestPlatformAtomicWriteFile_OverwritesExisting verifies that the platform
// helper replaces existing files instead of silently skipping them.
func TestPlatformAtomicWriteFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite_test")

	first := bytes.NewReader([]byte("first"))
	if err := PlatformAtomicWriteFile(path, first, 0644); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	second := bytes.NewReader([]byte("second"))
	if err := PlatformAtomicWriteFile(path, second, 0644); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed reading result: %v", err)
	}

	if string(contents) != "second" {
		t.Fatalf("expected file to be overwritten, got %q", string(contents))
	}
}
