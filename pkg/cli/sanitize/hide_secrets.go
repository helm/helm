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

	"helm.sh/helm/v3/pkg/release"

	"gopkg.in/yaml.v2"
)

const (
	hiddenSecretValue = "[HIDDEN]"
)

// HideManifestSecrets sanitizes release manifest and replaces it.
// Manifest cannot be applied after this operation.
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
			return "", err
		}

		if isSecret(resourceMap) {
			r = hideSecretData(r, resourceMap)
		}

		outRes = append(outRes, r)
	}

	return strings.Join(outRes, "\n---"), nil
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

func hideSecretData(raw string, resource map[string]interface{}) string {
	dataRaw, ok := resource["data"].(map[interface{}]interface{})
	if !ok || len(dataRaw) == 0 {
		return raw
	}

	data := toMapOfStrings(dataRaw)

	lines := strings.Split(raw, "\n")
	outLines := make([]string, len(lines))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// If line is part of secret.data, sanitize line by replacing the value part
		if key, matches := matchKeyValPair(data, trimmed); matches {
			sanitizedLine := strings.Replace(line, trimmed, formatHiddenValue(key), 1)
			outLines[i] = sanitizedLine
			continue
		}

		outLines[i] = line
	}

	return strings.Join(outLines, "\n")
}

func toMapOfStrings(rawMap map[interface{}]interface{}) map[string]string {
	stringsMap := make(map[string]string, len(rawMap))
	for k, v := range rawMap {
		key, ok := k.(string)
		if !ok {
			continue
		}
		val, ok := v.(string)
		if !ok {
			continue
		}
		stringsMap[key] = val
	}
	return stringsMap
}

// matchKeyValPair checks if data contains joined key value pair in format
// `key: value` equal to specified string.
// Returns key with which string matched and indicator if it matched any.
func matchKeyValPair(data map[string]string, str string) (string, bool) {
	for k, v := range data {
		joined := joinKeyVal(k, v)

		if joined == str {
			return k, true
		}
	}

	return "", false
}

func joinKeyVal(key, val string) string {
	return fmt.Sprintf("%s: %s", key, val)
}

func formatHiddenValue(key string) string {
	return joinKeyVal(key, hiddenSecretValue)
}
