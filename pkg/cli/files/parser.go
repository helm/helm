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

package files

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ParseIntoString parses a include-file line and merges the result into dest.
func ParseIntoString(s string, dest map[string]string) error {
	for _, val := range strings.Split(s, ",") {
		val = strings.TrimSpace(val)
		splt := strings.SplitN(val, "=", 2)

		if len(splt) != 2 {
			return errors.New("Could not parse line")
		}

		name := strings.TrimSpace(splt[0])
		path := strings.TrimSpace(splt[1])
		dest[name] = path
	}

	return nil
}

//ParseGlobIntoString parses an include-dir file line and merges all files found into dest.
func ParseGlobIntoString(g string, dest map[string]string) error {
	globs := make(map[string]string)
	err := ParseIntoString(g, globs)
	if err != nil {
		return err
	}
	for k, g := range globs {
		if !strings.Contains(g, "*") {
			// force glob style on simple directories
			g = strings.TrimRight(g, "/") + "/*"
		}

		paths, err := filepath.Glob(g)
		if err != nil {
			return err
		}

		k = strings.TrimRight(k, "/")
		for _, path := range paths {
			dest[fmt.Sprintf("%s/%s", k, filepath.Base(path))] = path
		}
	}

	return nil
}
