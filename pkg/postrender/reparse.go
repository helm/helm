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

package postrender

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// Reparse attempts to split a YAML stream as returned by a post-rendered back into a map of files
//
// Elsewhere in Helm, it treats individual YAMLs as filename/content pairs. The post-render
// is inserted into the middle of that context. Thus, when a post-render returns, we need
// a way to convert it back into a map[string]string. There are no assumptions about
// what the filename looks like when it comes back from the postrenderer, so we can take
// some liberties with naming here that we cannot take in other contexts.
//
// Note that the YAML specification is very clear that the string '\n---\n' is a document
// split sequence. So we can cheaply process using that method. Also we rely on the
// Kubernetes requirement that metadata.name is a required field for all valid Kubernetes
// resource instances, as are apiVersion and kind.
func Reparse(manifest []byte) (map[string]string, error) {
	sep := []byte("\n---\n")
	manifests := bytes.Split(manifest, sep)
	files := map[string]string{}

	for _, resource := range manifests {
		if s := strings.TrimSpace(string(resource)); s == "" {
			continue
		}
		h := &header{}
		if err := yaml.Unmarshal(resource, h); err != nil {
			return files, errors.Wrap(err, "manifest returned from post render is not well-formed")
		}

		// Name and Kind are required on every manifest
		if h.Kind == "" {
			return files, fmt.Errorf("manifest returned by post-render has no kind:\n%s", resource)
		}
		if h.Metadata.Name == "" {
			return files, fmt.Errorf("manifest returned by post-render has no name:\n%s", resource)
		}
		name := h.filename()
		if _, ok := files[name]; ok {
			return files, fmt.Errorf("two or more post-rendered objects have the name %q", name)
		}
		files[name] = string(resource)
	}
	return files, nil
}

type header struct {
	APIVersion string `json:"apiVersion"`
	Kind       string
	Metadata   struct {
		Name string
	}
}

func (h *header) filename() string {
	name := ""
	if h.APIVersion != "" {
		name = h.APIVersion + "."
	}
	return fmt.Sprintf("%s%s.%s.yaml", name, h.Kind, h.Metadata.Name)
}
