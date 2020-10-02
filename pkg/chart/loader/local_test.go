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

package loader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandLocalPath(t *testing.T) {
	need := require.New(t)
	is := assert.New(t)

	glob, err := ExpandLocalPath("glob", "testdata/frobnitz/*.yaml")
	need.NoError(err)
	need.Contains(glob, "glob/Chart.yaml")
	need.Contains(glob, "glob/values.yaml")
	is.Equal("testdata/frobnitz/Chart.yaml", glob["glob/Chart.yaml"])
	is.Equal("testdata/frobnitz/values.yaml", glob["glob/values.yaml"])

	dir, err := ExpandLocalPath("dir", "testdata/albatross/")
	need.NoError(err)
	need.Contains(dir, "dir/Chart.yaml")
	need.Contains(dir, "dir/values.yaml")
	is.Equal("testdata/albatross/Chart.yaml", dir["dir/Chart.yaml"])
	is.Equal("testdata/albatross/values.yaml", dir["dir/values.yaml"])

	file, err := ExpandLocalPath("file", "testdata/albatross/Chart.yaml")
	need.NoError(err)
	need.Contains(file, "file")
	is.Equal("testdata/albatross/Chart.yaml", file["file"])

}
