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

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIntoString(t *testing.T) {
	need := require.New(t)
	is := assert.New(t)

	dest := make(map[string]string)
	goodFlag := "foo.txt=../foo.txt"
	anotherFlag := " bar.txt=~/bar.txt, baz.txt=/path/to/baz.txt"

	err := ParseIntoString(goodFlag, dest)
	need.NoError(err)

	err = ParseIntoString(anotherFlag, dest)
	need.NoError(err)

	is.Contains(dest, "foo.txt")
	is.Contains(dest, "bar.txt")
	is.Contains(dest, "baz.txt")

	is.Equal(dest["foo.txt"], "../foo.txt", "foo.txt not mapped properly")
	is.Equal(dest["bar.txt"], "~/bar.txt", "bar.txt not mapped properly")
	is.Equal(dest["baz.txt"], "/path/to/baz.txt", "baz.txt not mapped properly")

	overwriteFlag := "foo.txt=../new_foo.txt"
	err = ParseIntoString(overwriteFlag, dest)
	need.NoError(err)

	is.Equal(dest["foo.txt"], "../new_foo.txt")

	badFlag := "empty.txt"
	err = ParseIntoString(badFlag, dest)
	is.NotNil(err)
}

func TestParseGlobIntoString(t *testing.T) {
	need := require.New(t)
	is := assert.New(t)

	dest := make(map[string]string)
	globFlagSlash := "glob/=testdata/foo/foo.*"
	dirFlagNoSlash := "dir=testdata/foo/"

	err := ParseGlobIntoString(globFlagSlash, dest)
	need.NoError(err)
	need.Contains(dest, "glob/foo.txt")
	is.Equal("testdata/foo/foo.txt", dest["glob/foo.txt"])

	err = ParseGlobIntoString(dirFlagNoSlash, dest)
	need.NoError(err)
	need.Contains(dest, "dir/foo.txt")
	need.Contains(dest, "dir/bar.txt")
	is.Equal("testdata/foo/foo.txt", dest["dir/foo.txt"])
	is.Equal("testdata/foo/bar.txt", dest["dir/bar.txt"])
}
