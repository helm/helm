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

// MergeValues collects configuration values from multiple sources specified using input flags, and merges them into a
// single YAML document.
//
// Sources are applied in the following order (earlier sources are overwritten by later ones if the same key appears
// more than once):
//
// 1. -d / --values-directory   : Values files from one or more directories
// 2. -f / --values             : Values files or URLs
// 3.      --set-json           : Values provided as raw JSON
// 4.      --set                : Inline key=value pairs
// 5.      --set-string         : Inline key=value pairs (values always treated as strings)
// 6.      --set-file           : Values read from file contents
// 7.      --set-literal        : Values provided as raw string literals
//
// Precedence order: 1 < 2 < 3 < 4 < 5 < 6 < 7 (higher number = higher precedence).
//
// For example, if `captain=luffy` is set via --values-directory (1) and `captain=usopp` is set via --values (2), the
// final merged value for `captain` will be "usopp".
//
// ---
//
// Additional context: The default values.
//
//   - Helm reads values from the chart's `values.yaml` file, if it exists, by default.
//   - These default values will have precedence 0 (lowest), i.e., values specified via any of the above-mentioned flags
//     will override the default values.
//   - If the `values.yaml` file is also specified via -f/--values, the values can override the ones from
//     -d/--values-directory. However, in that case, they are no longer considered "default" values.
//
// Note: This is not part of this function. But it is important for understanding the overall precedence order.
func (opts *Options) MergeValues(p getter.Providers) (map[string]any, error) {
	base := map[string]any{}

	var valuesFiles []string

	// 1. User specified directory(s) via -d/--values-directory.
	for _, dir := range opts.ValuesDirectories {
		// Recursive list of YAML files in input values directory
		files, err := listFilesRecursive(dir, `.yaml`)
		if err != nil {
			// Error already wrapped
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

// listFilesRecursive walks a directory tree recursively and returns a list of files, sorted lexicographically.
//
// If an extension is specified (e.g., ".yaml"), only files with that extension are included.
// If extension is an empty string, all files are returned.
//
// Example: (directory="foo", extension=".yaml")
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
func listFilesRecursive(directory, extension string) ([]string, error) {
	var files []string

	// Walk the directory tree in lexical order. For the above example, this will visit:
	// 1. foo/bar
	// 2. foo/bar/bar.yaml
	// 3. foo/baz
	// 4. foo/baz/baz.yaml
	// 5. foo/baz/qux.yaml
	// 6. foo/baz.txt
	// 7. foo/foo.yaml
	//
	// The inner function filters the files based on the specified extension. For eg., if extension=".yaml", only the
	// following files are collected, in the order:
	// - foo/bar/bar.yaml
	// - foo/baz/baz.yaml
	// - foo/baz/qux.yaml
	// - foo/foo.yaml
	//
	// Note: Since filepath.WalkDir walks in lexical order, the returned list of files is also sorted lexicographically.
	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to read file info for %q: %w", path, err)
		}

		// Collect files matching the extension (or all if extension is empty). Skip directories.
		if !d.IsDir() && (extension == "" || filepath.Ext(path) == extension) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files in directory %q: %w", directory, err)
	}

	return files, nil
}
