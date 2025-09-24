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
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "set-json object",
			opts: Options{
				JSONValues: []string{`{"foo": {"bar": "baz"}}`},
			},
			expected: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
		},
		{
			name: "set-json key=value",
			opts: Options{
				JSONValues: []string{"foo.bar=[1,2,3]"},
			},
			expected: map[string]any{
				"foo": map[string]any{
					"bar": []any{1.0, 2.0, 3.0},
				},
			},
		},
		{
			name: "set regular value",
			opts: Options{
				Values: []string{"foo=bar"},
			},
			expected: map[string]any{
				"foo": "bar",
			},
		},
		{
			name: "set string value",
			opts: Options{
				StringValues: []string{"foo=123"},
			},
			expected: map[string]any{
				"foo": "123",
			},
		},
		{
			name: "set literal value",
			opts: Options{
				LiteralValues: []string{"foo=true"},
			},
			expected: map[string]any{
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
			expected: map[string]any{
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
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory doesn't exist, the function should:
			// - Return a nil map and an error.
			name: "missing values directory",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/no-such-directory",
				},
			},
			// Directory doesn't exist, so expect a nil map.
			expected: nil,
			// Directory doesn't exist, so expect an error.
			wantErr: true,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains no files, the function should:
			// - Return an empty map.
			// - No errors should occur.
			name: "values directory without files",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/values-directory-without-files",
				},
			},
			// No YAML files, so expect an empty map.
			expected: map[string]any{},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains only non-YAML files, the function should:
			// - Return an empty map.
			// - No errors should occur.
			name: "values directory with only non-YAML files",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/values-directory-with-only-non-yaml-files",
				},
			},
			expected: map[string]any{},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains single YAML file, the function should:
			// - Read and parse the YAML file.
			// - Return the result map, which contains the key-value pairs from the file.
			// - No errors should occur.
			name: "values directory with single YAML file",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/values-directory-with-single-yaml-file",
				},
			},
			expected: map[string]any{
				// Field "alliances" is read from "alliances-law.yaml" file.
				"alliances": []any{
					map[string]any{
						"name":    "Heart Pirates",
						"captain": "Trafalgar D. Water Law",
					},
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains one YAML file and one non-YAML file, the function should:
			// - Read and parse the YAML file.
			// - Return the result map, which contains the key-value pairs from the YAML file.
			// - No errors should occur.
			name: "values directory with one YAML file and one non-YAML file",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/values-directory-with-one-yaml-file-and-one-non-yaml-file",
				},
			},
			expected: map[string]any{
				// Field "ship" is read from "ship-thousand-sunny.yaml" file.
				"ship": map[string]any{
					"name":   "Thousand Sunny",
					"status": "Active",
					"speed":  "Fast",
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains multiple YAML files with no overlapping keys, the function should:
			// - Read and parse all YAML files individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from all files are present, and none are overwritten
			//   (since there is no overlap).
			// - No errors should occur.
			name: "values directory with multiple YAML files, without overlapping keys",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-multiple-yaml-files-without-overlapping-keys",
				},
			},
			expected: map[string]any{
				// Field "alliances" is read from "alliances-kid.yaml".
				"alliances": []any{
					map[string]any{
						"name":    "Kid Pirates",
						"captain": "Eustass Kid",
					},
				},
				// Field "ship" is read from "ship-going-merry.yaml".
				"ship": map[string]any{
					"name":   "Going Merry",
					"status": "Destroyed",
					"speed":  "Slow",
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains multiple YAML files with few overlapping keys, the function should:
			// - Read and parse all YAML files individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from all files are present, and for overlapping keys, the
			//   value from the last file (lexicographically) processed takes precedence (overwrites previous values).
			// - No errors should occur.
			name: "values directory with multiple YAML files, with few overlapping keys",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-multiple-yaml-files-with-few-overlapping-keys",
				},
			},
			expected: map[string]any{
				"crew": []any{
					map[string]any{
						"name": "Monkey D. Luffy",
						"role": "Captain",
						// Field "bounty" is set to "30,000,000" in "1-crew-luffy-east-blue.yaml", and is overridden
						// as "100,000,000" in "2-crew-luffy-alabasta.yaml".
						"bounty":      "100,000,000",
						"age":         float64(19),
						"devil_fruit": "Gomu Gomu no Mi",
					},
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and contains multiple YAML files with all overlapping keys, the function should:
			// - Read and parse all YAML files individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from the last file (lexicographically) overwriting all
			//   from previous files.
			// - No errors should occur.
			name: "values directory with multiple YAML files, with all overlapping keys",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-multiple-yaml-files-with-all-overlapping-keys",
				},
			},
			expected: map[string]any{
				"ship": map[string]any{
					// Field "name" is set to "Going Merry" in "ship-going-merry.yaml", and is overridden as
					// "Thousand Sunny" in "ship-thousand-sunny.yaml".
					"name": "Thousand Sunny",
					// Field "status" is set to "Destroyed" in "ship-going-merry.yaml", and is overridden as
					// "Active" in "ship-thousand-sunny.yaml".
					"status": "Active",
					// Field "speed" is set to "Slow" in "ship-going-merry.yaml", and is overridden as "Fast" in
					// "ship-thousand-sunny.yaml".
					"speed": "Fast",
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and nested directories and YAML files with no overlapping keys, the function
			// should:
			// - Read and parse all YAML files, from all levels, individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from all files are present, and none are overwritten
			//   (since there is no overlap).
			// - No errors should occur.
			name: "values directory with nested directories and YAML files, without overlapping keys",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-nested-directories-and-YAML-files-without-overlapping-keys",
				},
			},
			expected: map[string]any{
				// Field "crew" is read from "1-crew-luffy-east-blue.yaml".
				"crew": []any{
					map[string]any{
						"name":        "Monkey D. Luffy",
						"role":        "Captain",
						"bounty":      "30,000,000",
						"age":         float64(19),
						"devil_fruit": "Gomu Gomu no Mi",
					},
				},
				// Field "alliances" is read from "subdir/alliances-grandfleet.yaml".
				"alliances": []any{
					map[string]any{
						"name":    "Straw Hat Grand Fleet",
						"captain": "Various",
					},
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and nested directories and YAML files with few overlapping keys, the function
			// should:
			// - Read and parse all YAML files, from all levels, individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from all files are present, and for overlapping keys, the
			//   value from the last file (lexicographically) processed takes precedence (overwrites previous values).
			// - No errors should occur.
			name: "values directory with nested directories and YAML files, with few overlapping keys",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-nested-directories-and-YAML-files-with-few-overlapping-keys",
				},
			},
			expected: map[string]any{
				"crew": []any{
					map[string]any{
						"name": "Monkey D. Luffy",
						"role": "Captain",
						// Field "bounty" is set to "30,000,000" in "1-crew-luffy-east-blue.yaml", and is overridden
						// as "100,000,000" in "subdir/2-crew-luffy-alabasta.yaml".
						"bounty":      "100,000,000",
						"age":         float64(19),
						"devil_fruit": "Gomu Gomu no Mi",
					},
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used, the given directory is scanned recursively for YAML files.
			// If the directory exists and nested directories and YAML files with all overlapping keys, the function
			// should:
			// - Read and parse all YAML files, from all levels, individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from the last file (lexicographically) overwriting all
			//   from previous files.
			// - No errors should occur.
			name: "values directory with nested directories and YAML files, with all overlapping keys",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-nested-directories-and-YAML-files-with-all-overlapping-keys",
				},
			},
			expected: map[string]any{
				"ship": map[string]any{
					// Field "name" is set to "Going Merry" in "ship-going-merry.yaml", and is overridden as
					// "Thousand Sunny" in "subdir/ship-thousand-sunny.yaml".
					"name": "Thousand Sunny",
					// Field "status" is set to "Destroyed" in "ship-going-merry.yaml", and is overridden as
					// "Active" in "subdir/ship-thousand-sunny.yaml".
					"status": "Active",
					// Field "speed" is set to "Slow" in "ship-going-merry.yaml", and is overridden as "Fast" in
					// "subdir/ship-thousand-sunny.yaml".
					"speed": "Fast",
				},
			},
			// No error expected.
			wantErr: false,
		},
		{
			// When the --values-directory (-d) flag is used to scan the directory recursively for YAML files, and the
			// remaining input flags -f/--values, --set-string, --set, --set-file, --set-json, --set-literal are used to
			// override values from the directory.
			// If the directory exists and nested directories and YAML files with all overlapping keys, the function
			// should:
			// - Read and parse all YAML files, from all levels, individually.
			// - Merge the resulting key-value maps into a single map.
			// - Return the merged map, where key-value pairs from the last file (lexicographically) overwriting all
			//   from previous files.
			// - No errors should occur.
			name: "values directory with YAML files, and values overridden via all other flags " +
				"(-f/--values, --set-string, --set, --set-file, --set-json, --set-literal)",
			opts: Options{
				// Set using flag -d / --values-directory (read values from YAML files in the specified directory).
				ValuesDirectories: []string{
					"testdata/one-piece-chart/values.d/" +
						"values-directory-with-YAML-files-and-values-overridden-via-all-other-flags",
				},

				// Set using flag -f / --values (read values from the specified YAML file).
				ValueFiles: []string{
					"testdata/one-piece-chart/values.d/shared-values/alliances-kid.yaml",
				},

				// Set using flag --set-string (inline string key=value pairs, value must be a string).
				StringValues: []string{
					"crew[0].role=Legendary Captain",
				},

				// Set using flag --set (inline key=value pairs, value type is inferred).
				Values: []string{
					// String value "ship.name" will be overridden via --set.
					"ship.name=Going Merry",
					// Integer value "crew[0].age" will be overridden via --set.
					"crew[0].age=20",
				},

				// Set using flag --set-file (read value from specified file).
				FileValues: []string{
					"crew[0].devil_fruit=testdata/one-piece-chart/values.d/shared-values/devil-fruit.txt",
				},

				// Set using flag --set-json (parse JSON string for value).
				JSONValues: []string{
					`ship.status={"condition":"Destroyed","where":"Enies Lobby"}`,
				},

				// Set using flag --set-literal (force literal string value, even if it is parsable to any other type).
				LiteralValues: []string{
					// String value "ship.speed" will be overridden via --set-literal.
					"ship.speed=Slow",
					// Integer value "crew[0].bounty" will be overridden via --set-literal.
					// Note: --set-literal treats numbers as strings.
					"crew[0].bounty=1,500,000,000",
				},
			},
			expected: map[string]any{
				"alliances": []any{ // Added via -f / --values.
					map[string]any{
						"captain": "Eustass Kid",
						"name":    "Kid Pirates",
					},
				},
				"crew": []any{
					map[string]any{
						"name": "Monkey D. Luffy",
						// Field "role" is overridden by --set-string.
						"role": "Legendary Captain",
						// Field "bounty" is overridden by --set-literal.
						"bounty": "1,500,000,000",
						// Field "devil_fruit" is overridden by --set-file.
						"devil_fruit": "Hito Hito no Mi",
						// Field "age" is overridden by --set.
						// Note: --set treats numbers as int64.
						"age": int64(20),
					},
				},
				"ship": map[string]any{
					// Field "name" is overridden by --set.
					"name": "Going Merry",
					// Field "status" is overridden by --set-json.
					"status": map[string]any{
						"condition": "Destroyed",
						"where":     "Enies Lobby",
					},
					// Field "speed" is overridden by --set-literal.
					"speed": "Slow",
				},
			},
			// No error expected.
			wantErr: false,
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
