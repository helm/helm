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
	"strings"

	"oras.land/oras-go/v2/registry"
)

type reference struct {
	orasReference registry.Reference
	Registry      string
	Repository    string
	Tag           string
	Digest        string
}

// newReference will parse and validate the reference, and clean tags when
// applicable tags are only cleaned when plus (+) signs are present, and are
// converted to underscores (_) before pushing
// See https://github.com/helm/helm/issues/10166
func newReference(raw string) (result reference, err error) {
	// Remove oci:// prefix if it is there
	raw = strings.TrimPrefix(raw, OCIScheme+"://")

	// The sole possible reference modification is replacing plus (+) signs
	// present in tags with underscores (_). To do this properly, we first
	// need to identify a tag, and then pass it on to the reference parser
	// NOTE: Passing immediately to the reference parser will fail since (+)
	// signs are an invalid tag character, and simply replacing all plus (+)
	// occurrences could invalidate other portions of the URI
	lastIndex := strings.LastIndex(raw, "@")
	if lastIndex >= 0 {
		result.Digest = raw[(lastIndex + 1):]
		raw = raw[:lastIndex]
	}
	parts := strings.Split(raw, ":")
	if len(parts) > 1 && !strings.Contains(parts[len(parts)-1], "/") {
		tag := parts[len(parts)-1]

		if tag != "" {
			// Replace any plus (+) signs with known underscore (_) conversion
			newTag := strings.ReplaceAll(tag, "+", "_")
			raw = strings.ReplaceAll(raw, tag, newTag)
		}
	}

	result.orasReference, err = registry.ParseReference(raw)
	if err != nil {
		return result, err
	}
	result.Registry = result.orasReference.Registry
	result.Repository = result.orasReference.Repository
	result.Tag = result.orasReference.Reference
	return result, nil
}

func (r *reference) String() string {
	if r.Tag == "" {
		return r.orasReference.String() + "@" + r.Digest
	}
	return r.orasReference.String()
}
