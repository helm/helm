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
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/strvals"
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
		currentMap := map[string]interface{}{}

		bytes, err := readFile(filePath, p)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", filePath)
		}
		// Merge with the previous map
		if base, err = mergeMaps(base, currentMap); err != nil {
			return nil, errors.Errorf("failed merging values files: %s", err)
		}
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

func mergeMaps(a, b map[string]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(a))
	var err error
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		listKeyName, t := strvals.FindListRune(k)
		if listKeyName != "" { // to allow list overrides - no merging within lists
			// ignore list index if there is the plain key in the same list already
			if _, ok := b[listKeyName]; ok {
				continue
			}
			overrideIdx, err := t.ParseListIndex()

			if err != nil {
				return out, fmt.Errorf("invalid key format %s for list override - %s", k, err)
			}
			// we can't assume that baseValue exists because we still have to merge to the base layer
			baseValue, hasBaseValue := out[listKeyName]
			var bvList []interface{}
			var ok bool
			if hasBaseValue {
				if bvList, ok = baseValue.([]interface{}); !ok {
					return out, fmt.Errorf("invalid key %s - the underlying value in the base layer is not a list", k)
				}
			}
			if bvList == nil {
				// if there is no underlying list, preserve the index override for the base values
				out[k] = v
			} else {
				// validate index and list merge operation
				if len(bvList)-1 < overrideIdx {
					return out, fmt.Errorf("invalid key format %s - index %d does not exist in the destination list", listKeyName, overrideIdx)
				}
				// copy the list so that we don't mutate the input
				destination := make([]interface{}, len(bvList))
				copy(destination, bvList)
				destination[overrideIdx] = v
				out[listKeyName] = destination
				continue
			}
		} else if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k], err = mergeMaps(bv, v)
					if err != nil {
						return out, fmt.Errorf("failed to merge values - %s", err)
					}
					continue
				}
			}
		}
		out[k] = v
	}
	return out, err
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
