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

package manifest

// ManifestOrderWeight is the label name for a manifest
const ManifestOrderWeight = "helm.sh/order-weight"

// Weight represents the deployment order of a manifest
type Weight struct {
	Chart    uint32
	Manifest uint32
}

// SimpleHead defines what the structure of the head of a manifest file
type SimpleHead struct {
	Version  string `json:"apiVersion"`
	Kind     string `json:"kind,omitempty"`
	Metadata *struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata,omitempty"`
}

// Manifest represents a manifest file, which has a name and some content.
type Manifest struct {
	Name    string
	Content string
	Head    *SimpleHead
	Weight  *Weight
}
