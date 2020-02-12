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

// package postrender contains an interface that can be implemented for custom
// post-renderers and an exec implementation that can be used for arbitrary
// binaries and scripts
package postrender

import "bytes"

type PostRenderer interface {
	// Run expects a single buffer filled with Helm rendered manifests. It
	// expects the modified results to be returned on a separate buffer or an
	// error if there was an issue or failure while running the post render step
	Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error)
}
