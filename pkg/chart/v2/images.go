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

package v2

import "sigs.k8s.io/yaml"

// ImagesAnnotation is the Chart.yaml annotation key that lists the container
// images used by a chart, as defined by HIP-0015.
const ImagesAnnotation = "helm.sh/images"

// Image describes a container image used by a chart.
//
// Fields not defined by HIP-0015 (e.g. Artifact Hub's "whitelisted") are ignored.
type Image struct {
	// Name is the name of the image.
	Name string `json:"name" yaml:"name"`
	// Image is the full image reference, including registry, repository and tag or digest.
	Image string `json:"image" yaml:"image"`
	// Dependency is the name of the chart dependency that uses the image, if any.
	Dependency string `json:"dependency,omitempty" yaml:"dependency,omitempty"`
}

// Validate checks that the image declares the fields required by HIP-0015.
func (i *Image) Validate() error {
	if i == nil {
		return ValidationError("images must not contain empty or null nodes")
	}
	i.Name = sanitizeString(i.Name)
	i.Image = sanitizeString(i.Image)
	i.Dependency = sanitizeString(i.Dependency)
	if i.Name == "" {
		return ValidationErrorf("image %q must have a name", i.Image)
	}
	// The image reference format (registry, tag, digest) is intentionally not
	// validated: HIP-0015 defines no grammar for it, and Kubernetes itself only
	// requires a non-empty value, leaving parsing to the container runtime at
	// pull time. Consumers that need a parsed reference should parse it themselves.
	// See https://github.com/kubernetes/kubernetes/blob/b50971128dc04458ff644fbcad28fe7a85e682b1/pkg/apis/core/validation/validation.go#L4020
	if i.Image == "" {
		return ValidationErrorf("image %q must have an image reference", i.Name)
	}
	return nil
}

// Images parses the helm.sh/images annotation.
//
// It returns nil and no error when the annotation is not set. An annotation
// that is not a valid YAML list, or an entry missing a required field,
// returns an error.
func (md *Metadata) Images() ([]*Image, error) {
	if md == nil {
		return nil, nil
	}
	raw, ok := md.Annotations[ImagesAnnotation]
	if !ok {
		return nil, nil
	}
	var images []*Image
	if err := yaml.Unmarshal([]byte(raw), &images); err != nil {
		return nil, ValidationErrorf("annotation %q is not a valid list of images: %s", ImagesAnnotation, err)
	}
	for _, img := range images {
		if err := img.Validate(); err != nil {
			return nil, err
		}
	}
	return images, nil
}
