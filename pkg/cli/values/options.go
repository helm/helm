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
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/strvals"
)

// Options captures the different ways to specify values
type Options struct {
	ValuesDirectories []string // -d / --values-directory
	ValueFiles        []string // -f / --values
	StringValues      []string //      --set-string
	Values            []string //      --set
	FileValues        []string //      --set-file
	JSONValues        []string //      --set-json
	LiteralValues     []string //      --set-literal
}

// MergeValues merges values from multiple sources according to Helm's precedence rules.
//
// The following list is ordered from lowest to highest precedence; items lower in the list override those above.
// i.e., values from sources later in the list take precedence over earlier ones:
//
//  1. -d / --values-directory	: Values files from one or more directories.
//  2. -f / --values			: Values files or URLs.
//  3. --set-json				: Values provided as raw JSON.
//  4. --set					: Inline key=value pairs.
//  5. --set-string				: Inline key=value pairs (values always treated as strings).
//  6. --set-file				: Values read from file contents.
//  7. --set-literal			: Values provided as raw string literals.
//
// For example, if `captain=Luffy` is set via --values-directory (1) and `captain=Usopp` is set via --values (2), the
// final merged value for `captain` will be "Usopp".
//
// ---
//
// In case any supported flag is specified multiple times, the latter occurrence has higher precedence, i.e. overrides
// the former.
//
// For example, for `--set captain=Luffy --set captain=Usopp`, the final merged value for `captain` will be "Usopp".
//
// This applies to all values flags (-d/--values-directory, -f/--values, --set-json, --set, --set-string, --set-file,
// and --set-literal).
//
// ---
//
// Additional context: The precedence of default values.
//
//   - By default, Helm reads values from the chart’s values.yaml file (if present).
//   - These default values have the lowest precedence (level 0), meaning any values specified via the above-mentioned
//     flags will override them.
//   - If the same values.yaml file is explicitly provided using -f/--values, its values can override those loaded via
//     -d/--values-directory. However, in that case, they are no longer considered default values.
//
// Note: Default values are not handled by this function, but understanding their precedence is important for the
// overall behavior.
func (opts *Options) MergeValues(p getter.Providers) (map[string]any, error) {
	base := map[string]any{}

	var valuesFiles []string

	// 1. User specified directory(s) via -d/--values-directory.
	for _, dir := range opts.ValuesDirectories {
		// Recursively find all .yaml files in the directory.
		files, err := listYAMLFilesRecursively(dir)
		if err != nil {
			// Error is already wrapped.
			return nil, err
		}

		valuesFiles = append(valuesFiles, files...)
	}

	// 2. User specified values files via -f/--values.
	valuesFiles = append(valuesFiles, opts.ValueFiles...)

	for _, filePath := range valuesFiles {
		raw, err := readFile(filePath, p)
		if err != nil {
			return nil, err
		}
		currentMap, err := loader.LoadValues(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
		}
		// Merge with the previous map
		base = loader.MergeMaps(base, currentMap)
	}

	// 3. User specified a value via --set-json.
	for _, value := range opts.JSONValues {
		trimmedValue := strings.TrimSpace(value)
		if len(trimmedValue) > 0 && trimmedValue[0] == '{' {
			// If value is JSON object format, parse it as map
			var jsonMap map[string]any
			if err := json.Unmarshal([]byte(trimmedValue), &jsonMap); err != nil {
				return nil, fmt.Errorf("failed parsing --set-json data JSON: %s", value)
			}
			base = loader.MergeMaps(base, jsonMap)
		} else {
			// Otherwise, parse it as key=value format
			if err := strvals.ParseJSON(value, base); err != nil {
				return nil, fmt.Errorf("failed parsing --set-json data %s", value)
			}
		}
	}

	// 4. User specified a value via --set.
	for _, value := range opts.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return nil, fmt.Errorf("failed parsing --set data: %w", err)
		}
	}

	// 5. User specified a value via --set-string.
	for _, value := range opts.StringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return nil, fmt.Errorf("failed parsing --set-string data: %w", err)
		}
	}

	// 6. User specified a value via --set-file.
	for _, value := range opts.FileValues {
		reader := func(rs []rune) (any, error) {
			bytes, err := readFile(string(rs), p)
			if err != nil {
				return nil, err
			}
			return string(bytes), err
		}
		if err := strvals.ParseIntoFile(value, base, reader); err != nil {
			return nil, fmt.Errorf("failed parsing --set-file data: %w", err)
		}
	}

	// 7. User specified a value via --set-literal.
	for _, value := range opts.LiteralValues {
		if err := strvals.ParseLiteralInto(value, base); err != nil {
			return nil, fmt.Errorf("failed parsing --set-literal data: %w", err)
		}
	}

	return base, nil
}

// readFile load a file from stdin, the local directory, or a remote file with a url.
func readFile(filePath string, p getter.Providers) ([]byte, error) {
	if strings.TrimSpace(filePath) == "-" {
		return io.ReadAll(os.Stdin)
	}
	u, err := url.Parse(filePath)
	if err != nil {
		return nil, err
	}

	// FIXME: maybe someone handle other protocols like ftp.
	g, err := p.ByScheme(u.Scheme)
	if err != nil {
		return os.ReadFile(filePath)
	}
	data, err := g.Get(filePath, getter.WithURL(filePath))
	if err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// listYAMLFilesRecursively walks a directory tree and returns a lexicographically sorted list of all YAML files.
//
// Example: (dir="foo")
//
//		foo/
//		├── bar/
//		│    └── bar.yaml
//		├── baz/
//		│    ├── baz.yaml
//		│    └── qux.yaml
//		├── baz.txt
//		└── foo.yaml
//
//	 Result: ["foo/bar/bar.yaml", "foo/baz/baz.yaml", "foo/baz/qux.yaml", "foo/foo.yaml"]
func listYAMLFilesRecursively(dir string) ([]string, error) {
	var files []string

	// Check if the directory exists and is a directory.
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to access values directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", dir)
	}

	// Walk the directory tree in lexical order. For the above example, this will visit:
	// 1. foo/bar
	// 2. foo/bar/bar.yaml
	// 3. foo/baz
	// 4. foo/baz/baz.yaml
	// 5. foo/baz/qux.yaml
	// 6. foo/baz.txt
	// 7. foo/foo.yaml
	//
	// The inner function filters the “.yaml” files as follows:
	// 1. foo/bar/bar.yaml
	// 2. foo/baz/baz.yaml
	// 3. foo/baz/qux.yaml
	// 4. foo/foo.yaml
	//
	// Note: Since filepath.WalkDir walks in lexical order, the returned list of files is also sorted lexicographically.
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %q: %w", path, err)
		}

		// Collect YAML files (.yaml or .yml, case-insensitive), skipping directories.
		if !d.IsDir() && isYamlFileExtension(d.Name()) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files in directory %q: %w", dir, err)
	}

	return files, nil
}

// isYamlFileExtension checks if the given file name has a YAML file extension. It returns true for files ending with
// .yaml or .yml (case-insensitive).
func isYamlFileExtension(fileName string) bool {
	// Extract file extension and convert to lower case for case-insensitive comparison.
	ext := strings.ToLower(filepath.Ext(fileName))

	// Check for .yaml or .yml extensions.
	return ext == ".yaml" || ext == ".yml"
}
