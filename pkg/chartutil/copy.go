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

package chartutil

func copyMap(src map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(src))
	for k, v := range src {
		m[k] = v
	}
	return m
}

// stringCollectionsDeepCopy makes deep copy for string maps and lists.
// For other types performs shallow copy.
func stringCollectionsDeepCopy(src any) any {
	switch t := src.(type) {
	case map[string]any:
		r := make(map[string]interface{}, len(t))
		for k, v := range t {
			r[k] = stringCollectionsDeepCopy(v)
		}
		return r
	case []string:
		r := make([]string, len(t))
		copy(r, t)
		return r
	default:
		return t
	}
}
