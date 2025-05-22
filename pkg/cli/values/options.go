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
	"net/url"
	"os"
	"strings"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/strvals"
)

// Options captures the different ways to specify values
type Options struct {
	ValueFiles    []string // -f/--values
	StringValues  []string // --set-string
	Values        []string // --set
	FileValues    []string // --set-file
	JSONValues    []string // --set-json
	LiteralValues []string // --set-literal
}

// MergeValues merges values from files specified via -f/--values and directly
// via --set-json, --set, --set-string, or --set-file, marshaling them to YAML
func (opts *Options) MergeValues(p getter.Providers) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range opts.ValueFiles {
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

	// User specified a value via --set-json
	for _, value := range opts.JSONValues {
		trimmedValue := strings.TrimSpace(value)
		if len(trimmedValue) > 0 && trimmedValue[0] == '{' {
			// If value is JSON object format, parse it as map
			var jsonMap map[string]interface{}
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

	// User specified a value via --set
	for _, value := range opts.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return nil, fmt.Errorf("failed parsing --set data: %w", err)
		}
	}

	// User specified a value via --set-string
	for _, value := range opts.StringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return nil, fmt.Errorf("failed parsing --set-string data: %w", err)
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
			return nil, fmt.Errorf("failed parsing --set-file data: %w", err)
		}
	}

	// User specified a value via --set-literal
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
