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

package gitupdate

import (
	"fmt"
	"strings"
)

// parsePointer implements the token decoding rules from RFC 6901. An empty
// pointer addresses the document root.
func parsePointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("path %q must be an RFC 6901 JSON pointer beginning with '/'", pointer)
	}

	raw := strings.Split(pointer[1:], "/")
	segments := make([]string, len(raw))
	for i, segment := range raw {
		var decoded strings.Builder
		for j := 0; j < len(segment); j++ {
			if segment[j] != '~' {
				decoded.WriteByte(segment[j])
				continue
			}
			if j+1 >= len(segment) {
				return nil, fmt.Errorf("path %q contains an invalid '~' escape", pointer)
			}
			j++
			switch segment[j] {
			case '0':
				decoded.WriteByte('~')
			case '1':
				decoded.WriteByte('/')
			default:
				return nil, fmt.Errorf("path %q contains invalid escape ~%c", pointer, segment[j])
			}
		}
		segments[i] = decoded.String()
	}
	return segments, nil
}
