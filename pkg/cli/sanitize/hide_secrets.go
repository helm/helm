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

package sanitize

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/release"

	"gopkg.in/yaml.v3"
)

const (
	hiddenSecretValue = "[HIDDEN]"
)

// HideManifestSecrets sanitizes release manifest and replaces it.
// Manifest cannot be applied after this operation.
// Indentation and extra spaces in Secret's `data` and `stringData` sections can be impacted.
func HideManifestSecrets(r *release.Release) error {
	if r == nil {
		return nil
	}
	manifest, err := hideSecrets(r.Manifest)
	if err != nil {
		return err
	}

	r.Manifest = manifest

	return nil
}

// hideSecrets replaces values in Secrets in the chart manifest with
// `[HIDDEN]` value.
func hideSecrets(manifest string) (string, error) {
	resources := strings.Split(manifest, "\n---")
	outRes := make([]string, 0, len(resources))

	for _, r := range resources {
		var resourceMap map[string]interface{}
		err := yaml.Unmarshal([]byte(r), &resourceMap)
		if err != nil {
			return "", errors.Wrapf(err, "failed to unmarshal %q resource", tryToGetName(resourceMap))
		}

		if isSecret(resourceMap) {
			rs, err := hideSecretData(r)
			if err != nil {
				return "", errors.Wrapf(err, "failed to hide %q Secret data", tryToGetName(resourceMap))
			}
			r = rs
		}

		outRes = append(outRes, r)
	}

	return strings.Join(outRes, "\n---"), nil
}

func tryToGetName(resourceMap map[string]interface{}) string {
	metadata, ok := resourceMap["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return ""
	}

	return name
}

func isSecret(resource map[string]interface{}) bool {
	kind, ok := resource["kind"].(string)
	if !ok || kind != "Secret" {
		return false
	}

	apiVersion, ok := resource["apiVersion"].(string)
	if !ok || apiVersion != "v1" {
		return false
	}

	return true
}

func hideSecretData(raw string) (string, error) {
	lines := strings.Split(raw, "\n")
	outLines := make([]string, 0, len(lines))

	// To minimize impact of empty lines and custom indentation
	// we only marshal `data` and `secretData` sections of the Secrets
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		outLines = append(outLines, line)

		if strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "stringData:") {
			endLine := findSectionEnd(lines, i+1)
			sanitizedLines, err := sanitizeDataYaml(lines[i+1 : endLine+1])
			if err != nil {
				return "", errors.Wrap(err, "failed to sanitize Secret data")
			}
			outLines = append(outLines, sanitizedLines...)

			i = endLine
		}
	}

	return strings.Join(outLines, "\n"), nil
}

func sanitizeDataYaml(yamlLines []string) ([]string, error) {
	yamlData := strings.Join(yamlLines, "\n")
	data := &yaml.Node{}
	err := yaml.Unmarshal([]byte(yamlData), data)
	if err != nil {
		return nil, err
	}
	if len(data.Content) == 0 {
		return []string{}, nil
	}

	node := data.Content[0]
	sanitizeNode(node)

	// Try to preserve indentation of the data section
	indent := "  "
	if len(node.Content) > 1 {
		lineNum := node.Content[1].Line
		indent = takeWhitespace(yamlLines[lineNum-1])
	}

	sanitized, err := yaml.Marshal(node)
	if err != nil {
		return nil, err
	}
	str := strings.TrimSpace(string(sanitized))
	lines := strings.Split(str, "\n")

	for i := range lines {
		lines[i] = fmt.Sprintf("%s%s", indent, lines[i])
	}

	return lines, nil
}

func sanitizeNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}
	contents := node.Content

	for i := 1; i < len(contents); i += 2 {
		contents[i].Style = 0 // Erase literal style
		contents[i].SetString(hiddenSecretValue)
	}
}

func takeWhitespace(line string) string {
	buff := make([]byte, 0)
	for _, c := range line {
		switch c {
		case ' ', '\t':
			buff = append(buff, byte(c))
			continue
		}

		break
	}
	return string(buff)
}

func findSectionEnd(lines []string, start int) int {
	i := start
	for i < len(lines) && isDataLine(lines[i]) {
		i++
	}
	return i - 1
}

func isDataLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	return len(line) > 1 && (line[0] == byte('\t') || line[0] == byte(' '))
}
