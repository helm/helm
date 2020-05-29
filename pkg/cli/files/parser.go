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
