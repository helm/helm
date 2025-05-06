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
	Run(renderedFiles map[string]string) (modifiedFiles map[string]string, err error)
}
