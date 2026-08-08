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

package rules

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// yaml11Booleans are the plain scalars that YAML 1.1 resolves to a boolean.
// Values are parsed with sigs.k8s.io/yaml, which speaks YAML 1.1, so such a
// scalar used as a mapping key becomes "true" or "false".
//
// The values passed to --set never go through a YAML parser, so the same key
// written there stays a string. The two ways of supplying a value therefore
// produce two different keys.
var yaml11Booleans = map[string]bool{
	"y": true, "Y": true, "yes": true, "Yes": true, "YES": true,
	"n": true, "N": true, "no": true, "No": true, "NO": true,
	"on": true, "On": true, "ON": true, "off": true, "Off": true, "OFF": true,
	"True": true, "False": true,
}

// booleanLikeKey describes a key that does not survive parsing under its own name.
type booleanLikeKey struct {
	path string
	line int
}

func (k booleanLikeKey) String() string {
	return fmt.Sprintf("%s (line %d)", k.path, k.line)
}

// findBooleanLikeKeys walks a values document and collects keys that YAML 1.1
// turns into a boolean. Quoted keys are left alone: quoting is the fix, and it
// already works.
func findBooleanLikeKeys(data []byte) ([]booleanLikeKey, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Parse errors are reported by the values rule; nothing to add here.
		return nil, nil
	}
	var found []booleanLikeKey
	for _, n := range doc.Content {
		walkKeys(n, "", &found)
	}
	return found, nil
}

func walkKeys(n *yaml.Node, path string, found *[]booleanLikeKey) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			name := k.Value
			child := name
			if path != "" {
				child = path + "." + name
			}
			if k.Style == 0 && yaml11Booleans[name] {
				*found = append(*found, booleanLikeKey{path: child, line: k.Line})
			}
			walkKeys(v, child, found)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			walkKeys(c, fmt.Sprintf("%s[%d]", path, i), found)
		}
	}
}

// validateNoBooleanLikeKeys warns about keys that a template cannot reach by the
// name the chart author wrote.
func validateNoBooleanLikeKeys(valuesPath string) error {
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return nil // absence and unreadability are reported by other rules
	}
	found, err := findBooleanLikeKeys(data)
	if err != nil || len(found) == 0 {
		return nil
	}
	names := make([]string, 0, len(found))
	for _, k := range found {
		names = append(names, k.String())
	}
	return fmt.Errorf("key(s) %s are read as booleans and become %q or %q in the values; "+
		"a template cannot reach them under the original name and --set will not override them. Quote the key to keep it",
		strings.Join(names, ", "), "true", "false")
}
