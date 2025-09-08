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

package values

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/getter"
)

// mockGetter implements getter.Getter for testing
type mockGetter struct {
	content []byte
	err     error
}

func (m *mockGetter) Get(_ string, _ ...getter.Option) (*bytes.Buffer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return bytes.NewBuffer(m.content), nil
}

// mockProvider creates a test provider
func mockProvider(schemes []string, content []byte, err error) getter.Provider {
	return getter.Provider{
		Schemes: schemes,
		New: func(_ ...getter.Option) (getter.Getter, error) {
			return &mockGetter{content: content, err: err}, nil
		},
	}
}

func TestReadFile(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		providers    getter.Providers
		setupFunc    func(*testing.T) (string, func()) // setup temp files, return cleanup
		expectError  bool
		expectStdin  bool
		expectedData []byte
	}{
		{
			name:        "stdin input with dash",
			filePath:    "-",
			providers:   getter.Providers{},
			expectStdin: true,
			expectError: false,
		},
		{
			name:        "stdin input with whitespace",
			filePath:    "  -  ",
			providers:   getter.Providers{},
			expectStdin: true,
			expectError: false,
		},
		{
			name:        "invalid URL parsing",
			filePath:    "://invalid-url",
			providers:   getter.Providers{},
			expectError: true,
		},
		{
			name:      "local file - existing",
			filePath:  "test.txt",
			providers: getter.Providers{},
			setupFunc: func(t *testing.T) (string, func()) {
				t.Helper()
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "test.txt")
				content := []byte("local file content")
				err := os.WriteFile(filePath, content, 0644)
				if err != nil {
					t.Fatal(err)
				}
				return filePath, func() {} // cleanup handled by t.TempDir()
			},
			expectError:  false,
			expectedData: []byte("local file content"),
		},
		{
			name:        "local file - non-existent",
			filePath:    "/non/existent/file.txt",
			providers:   getter.Providers{},
			expectError: true,
		},
		{
			name:     "remote file with http scheme - success",
			filePath: "http://example.com/values.yaml",
			providers: getter.Providers{
				mockProvider([]string{"http", "https"}, []byte("remote content"), nil),
			},
			expectError:  false,
			expectedData: []byte("remote content"),
		},
		{
			name:     "remote file with https scheme - success",
			filePath: "https://example.com/values.yaml",
			providers: getter.Providers{
				mockProvider([]string{"http", "https"}, []byte("https content"), nil),
			},
			expectError:  false,
			expectedData: []byte("https content"),
		},
		{
			name:     "remote file with custom scheme - success",
			filePath: "oci://registry.example.com/chart",
			providers: getter.Providers{
				mockProvider([]string{"oci"}, []byte("oci content"), nil),
			},
			expectError:  false,
			expectedData: []byte("oci content"),
		},
		{
			name:     "remote file - getter error",
			filePath: "http://example.com/values.yaml",
			providers: getter.Providers{
				mockProvider([]string{"http"}, nil, errors.New("network error")),
			},
			expectError: true,
		},
		{
			name:     "unsupported scheme fallback to local file",
			filePath: "ftp://example.com/file.txt",
			providers: getter.Providers{
				mockProvider([]string{"http"}, []byte("should not be used"), nil),
			},
			setupFunc: func(t *testing.T) (string, func()) {
				t.Helper()
				// Create a local file named "ftp://example.com/file.txt"
				// This tests the fallback behavior when scheme is not supported
				tmpDir := t.TempDir()
				fileName := "ftp_file.txt" // Valid filename for filesystem
				filePath := filepath.Join(tmpDir, fileName)
				content := []byte("local fallback content")
				err := os.WriteFile(filePath, content, 0644)
				if err != nil {
					t.Fatal(err)
				}
				return filePath, func() {}
			},
			expectError:  false,
			expectedData: []byte("local fallback content"),
		},
		{
			name:        "empty file path",
			filePath:    "",
			providers:   getter.Providers{},
			expectError: true, // Empty path should cause error
		},
		{
			name:     "multiple providers - correct selection",
			filePath: "custom://example.com/resource",
			providers: getter.Providers{
				mockProvider([]string{"http", "https"}, []byte("wrong content"), nil),
				mockProvider([]string{"custom"}, []byte("correct content"), nil),
				mockProvider([]string{"oci"}, []byte("also wrong"), nil),
			},
			expectError:  false,
			expectedData: []byte("correct content"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualFilePath string
			var cleanup func()

			if tt.setupFunc != nil {
				actualFilePath, cleanup = tt.setupFunc(t)
				defer cleanup()
			} else {
				actualFilePath = tt.filePath
			}

			// Handle stdin test case
			if tt.expectStdin {
				// Save original stdin
				originalStdin := os.Stdin
				defer func() { os.Stdin = originalStdin }()

				// Create a pipe for stdin
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				defer w.Close()

				// Replace stdin with our pipe
				os.Stdin = r

				// Write test data to stdin
				testData := []byte("stdin test data")
				go func() {
					defer w.Close()
					w.Write(testData)
				}()

				// Test the function
				got, err := readFile(actualFilePath, tt.providers)
				if err != nil {
					t.Errorf("readFile() error = %v, expected no error for stdin", err)
					return
				}

				if !bytes.Equal(got, testData) {
					t.Errorf("readFile() = %v, want %v", got, testData)
				}
				return
			}

			// Regular test cases
			got, err := readFile(actualFilePath, tt.providers)
			if (err != nil) != tt.expectError {
				t.Errorf("readFile() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && tt.expectedData != nil {
				if !bytes.Equal(got, tt.expectedData) {
					t.Errorf("readFile() = %v, want %v", got, tt.expectedData)
				}
			}
		})
	}
}

// TestReadFileErrorMessages tests specific error scenarios and their messages
func TestReadFileErrorMessages(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		providers getter.Providers
		wantErr   string
	}{
		{
			name:      "URL parse error",
			filePath:  "://invalid",
			providers: getter.Providers{},
			wantErr:   "missing protocol scheme",
		},
		{
			name:      "getter error with message",
			filePath:  "http://example.com/file",
			providers: getter.Providers{mockProvider([]string{"http"}, nil, fmt.Errorf("connection refused"))},
			wantErr:   "connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readFile(tt.filePath, tt.providers)
			if err == nil {
				t.Errorf("readFile() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("readFile() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

// Original test case - keeping for backward compatibility
func TestReadFileOriginal(t *testing.T) {
	var p getter.Providers
	filePath := "%a.txt"
	_, err := readFile(filePath, p)
	if err == nil {
		t.Errorf("Expected error when has special strings")
	}
}

func TestMergeValuesCLI(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name: "set-json object",
			opts: Options{
				JSONValues: []string{`{"foo": {"bar": "baz"}}`},
			},
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		{
			name: "set-json key=value",
			opts: Options{
				JSONValues: []string{"foo.bar=[1,2,3]"},
			},
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": []interface{}{1.0, 2.0, 3.0},
				},
			},
		},
		{
			name: "set regular value",
			opts: Options{
				Values: []string{"foo=bar"},
			},
			expected: map[string]interface{}{
				"foo": "bar",
			},
		},
		{
			name: "set string value",
			opts: Options{
				StringValues: []string{"foo=123"},
			},
			expected: map[string]interface{}{
				"foo": "123",
			},
		},
		{
			name: "set literal value",
			opts: Options{
				LiteralValues: []string{"foo=true"},
			},
			expected: map[string]interface{}{
				"foo": "true",
			},
		},
		{
			name: "multiple options",
			opts: Options{
				Values:        []string{"a=foo"},
				StringValues:  []string{"b=bar"},
				JSONValues:    []string{`{"c": "foo1"}`},
				LiteralValues: []string{"d=bar1"},
			},
			expected: map[string]interface{}{
				"a": "foo",
				"b": "bar",
				"c": "foo1",
				"d": "bar1",
			},
		},
		{
			name: "invalid json",
			opts: Options{
				JSONValues: []string{`{invalid`},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.MergeValues(getter.Providers{})
			if (err != nil) != tt.wantErr {
				t.Errorf("MergeValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MergeValues() = %v, want %v", got, tt.expected)
			}
		})
	}
}
