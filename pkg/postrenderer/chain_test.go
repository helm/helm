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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRenderer is a simple PostRenderer used for chain testing. It appends
// its suffix to the input, or returns an error if configured to fail.
type fakeRenderer struct {
	suffix  string
	failErr error
}

func (f *fakeRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	out := bytes.NewBufferString(renderedManifests.String() + f.suffix)
	return out, nil
}

func TestChain_Empty(t *testing.T) {
	c := NewChain()
	out, err := c.Run(bytes.NewBufferString("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello", out.String())
}

func TestChain_Single(t *testing.T) {
	c := NewChain(&fakeRenderer{suffix: "-a"})
	out, err := c.Run(bytes.NewBufferString("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello-a", out.String())
}

func TestChain_Multiple_OrderPreserved(t *testing.T) {
	c := NewChain(
		&fakeRenderer{suffix: "-a"},
		&fakeRenderer{suffix: "-b"},
		&fakeRenderer{suffix: "-c"},
	)
	out, err := c.Run(bytes.NewBufferString("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello-a-b-c", out.String())
}

func TestChain_ErrorStopsPipeline(t *testing.T) {
	boom := errors.New("boom")
	c := NewChain(
		&fakeRenderer{suffix: "-a"},
		&fakeRenderer{failErr: boom},
		&fakeRenderer{suffix: "-c"},
	)
	out, err := c.Run(bytes.NewBufferString("hello"))
	require.Error(t, err)
	assert.Nil(t, out)
	assert.ErrorIs(t, err, boom)
}
