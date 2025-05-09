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

// Package postrender contains an interface that can be implemented for custom
// post-renderers and an exec implementation that can be used for arbitrary
// binaries and scripts
package postrender

type PostRenderer interface {
	// Run expects and returns a map of file names and their rendered contents
	// Example:
	// > map[string]string{
	// >   "templates/foo.yaml": "Kind: Pod..",
	// >   "templates/baz.yaml": "Kind: ConfigMap...",
	// > }
	// Note: In Helm 4, the format of the data passed to the post-renderer command
	// has changed in a backward-incompatible way. Helm 4 now passes a YAML-encoded
	// map of filenames to their rendered content.
	// Example:
	// > templates/foo.yaml: |
	// >   apiVersion: v1
	// >   kind: Pod
	// >   ...
	// > templates/bar.yaml: |
	// >   ...
	//
	// In contrast, Helm 3 passed a stream of YAML manifests (just the values of the map).
	// Example:
	// > # Source: templates/foo.yaml
	// > apiVersion: v1
	// > kind: Pod
	// > ...
	// > # Source: templates/bar.yaml
	// > ...
	//
	// This change allows the post-renderer to view, add, remove, and modify all rendered
	// files at once before they are sorted into hooks, manifests, and partials.
	Run(renderedFiles map[string]string) (modifiedFiles map[string]string, err error)
}
