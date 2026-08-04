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

package postrenderer

import (
	"bytes"
	"fmt"
)

// Chain is a PostRenderer that runs a sequence of PostRenderers, feeding the
// output of each renderer as the input to the next. This allows multiple
// post-renderer plugins to be composed into a single pipeline.
type Chain struct {
	renderers []PostRenderer
}

// NewChain creates a Chain that will run the given renderers in order. An
// empty chain is valid and behaves as a no-op renderer that returns its
// input unchanged.
func NewChain(renderers ...PostRenderer) *Chain {
	return &Chain{renderers: renderers}
}

func (c *Chain) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	manifests := renderedManifests
	for i, renderer := range c.renderers {
		var err error
		manifests, err = renderer.Run(manifests)
		if err != nil {
			return nil, fmt.Errorf("post-renderer %d/%d failed: %w", i+1, len(c.renderers), err)
		}
	}
	return manifests, nil
}
