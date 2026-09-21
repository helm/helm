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

package sequence

import (
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

// ParseStoredManifests parses a stored release manifest stream in input order.
func ParseStoredManifests(manifest string) ([]releaseutil.Manifest, error) {
	rawManifests := releaseutil.SplitManifests(manifest)
	keys := make([]string, 0, len(rawManifests))
	for key := range rawManifests {
		keys = append(keys, key)
	}
	sort.Sort(releaseutil.BySplitManifestsOrder(keys))

	manifests := make([]releaseutil.Manifest, 0, len(keys))
	for _, key := range keys {
		content := rawManifests[key]
		name := manifestSourcePath(content)
		if name == "" {
			name = key
		}

		var head releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(content), &head); err != nil {
			return nil, fmt.Errorf("YAML parse error on %s: %w", name, err)
		}

		manifests = append(manifests, releaseutil.Manifest{
			Name:    name,
			Content: content,
			Head:    &head,
		})
	}

	return manifests, nil
}

func manifestSourcePath(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if source, ok := strings.CutPrefix(line, "# Source: "); ok {
			return source
		}
	}
	return ""
}
