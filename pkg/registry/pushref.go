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

package registry

import (
	"fmt"
	"path"
	"strings"
)

func BuildPushRef(href, chartName, chartVersion string) (string, error) {
	ref, err := newReference(href)
	if err != nil {
		return "", err
	}

	if ref.Digest != "" {
		return "", fmt.Errorf("cannot push to a reference with a digest: %q. Only tags are allowed", href)
	}

	// Normalize chart version for tag comparison/build (registry tags cannot contain '+')
	normalizedVersion := strings.ReplaceAll(chartVersion, "+", "_")

	// Determine final tag:
	// - if href tag present, it must match normalized chart version
	// - else use chart version
	finalTag := normalizedVersion
	if ref.Tag != "" {
		if ref.Tag != normalizedVersion {
			return "", fmt.Errorf("tag %q does not match provided chart version %q", ref.Tag, chartVersion)
		}
		finalTag = ref.Tag
	}

	// Ensure repository ends with the chart name once (avoid duplication)
	finalRepo := ref.Repository
	if chartName != "" {
		last := chartName
		// Extract last segment of current repository path
		if idx := strings.LastIndex(finalRepo, "/"); idx >= 0 {
			last = finalRepo[idx+1:]
		} else {
			last = finalRepo
		}
		if last != chartName {
			finalRepo = path.Join(finalRepo, chartName)
		}
	}

	return fmt.Sprintf("%s/%s:%s", ref.Registry, finalRepo, finalTag), nil
}
