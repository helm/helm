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
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/strvals"
)

// Options captures the different ways to specify values
type Options struct {
	ValueFiles        []string // -f/--values
	ValuesDirectories []string // -d/--values-directory
	StringValues      []string // --set-string
	Values            []string // --set
	FileValues        []string // --set-file
	JSONValues        []string // --set-json
	LiteralValues     []string // --set-literal
}

// MergeValues merges values specified via any of the following flags, and marshals them to YAML:
// 1. -d/--values-directory - from values file(s) in the directory(s)
// 2. -f/--values			- from values file(s) or URL
// 3. 	 --set-json			- from input JSON
// 4. 	 --set				- from input key-value pairs
// 5. 	 --set-string		- from input key-value pairs, with string values, always
// 6. 	 --set-file			- from files
// 7. 	 --set-literal		- from input string literal
//
// The precedence order of inputs are 1 to 7, where 1 gets evaluated first and 7 last. i.e., If key1="val1" in inputs
// from --values-directory, and key1="val2" in --values, the second overwrites the first and the final value of key1
// is "val2". Similarly values from --set-json are replaced from that of --values, and so on.
func (opts *Options) MergeValues(p getter.Providers) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	var valuesFiles []string

	// User specified directory(s) via -d/--values-directory
	for _, dir := range opts.ValuesDirectories {
		// Recursive list of YAML files in input values directory
		files, err := recursiveListOfFilesInDir(dir, `.yaml`)
		if err != nil {
			// Error already wrapped
			return nil, err
		}

		valuesFiles = append(valuesFiles, files...)
	}

	// User specified values files via -f/--values
	valuesFiles = append(valuesFiles, opts.ValueFiles...)

	for _, filePath := range valuesFiles {
		currentMap := map[string]interface{}{}

		bytes, err := readFile(filePath, p)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", filePath)
		}
		// Merge with the previous map
		base = mergeMaps(base, currentMap)
	}

	// User specified a value via --set-json
	for _, value := range opts.JSONValues {
		if err := strvals.ParseJSON(value, base); err != nil {
			return nil, errors.Errorf("failed parsing --set-json data %s", value)
		}
	}

	// User specified a value via --set
	for _, value := range opts.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set data")
		}
	}

	// User specified a value via --set-string
	for _, value := range opts.StringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set-string data")
		}
	}

	// User specified a value via --set-file
	for _, value := range opts.FileValues {
		reader := func(rs []rune) (interface{}, error) {
			bytes, err := readFile(string(rs), p)
			if err != nil {
				return nil, err
			}
			return string(bytes), err
		}
		if err := strvals.ParseIntoFile(value, base, reader); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set-file data")
		}
	}

	// User specified a value via --set-literal
	for _, value := range opts.LiteralValues {
		if err := strvals.ParseLiteralInto(value, base); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set-literal data")
		}
	}

	return base, nil
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
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
	return data.Bytes(), err
}

// recursiveListOfFilesInDir lists the directory recursively, i.e., files in all nested directories.
// The list can be filtered by file extension. If no extension is specified, it returns all files.
//
// Result format: [<dir>/<file>, ..., <dir>/<sub-dir>/<file> ...]
func recursiveListOfFilesInDir(directory, extension string) ([]string, error) {
	var files []string

	// Traverse through the directory, recursively
	err := filepath.WalkDir(directory, func(path string, file fs.DirEntry, err error) error {
		// Check if accessing the file failed
		if err != nil {
			return errors.Wrapf(err, "failed to read info of file %q", path)
		}

		// When the file has the required extension, or when extension is not specified
		if !file.IsDir() && (extension == "" || filepath.Ext(path) == extension) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to recursively list files in directory %q", directory)
	}

	return files, nil
}
