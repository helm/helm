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

package engine

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

var cases = []struct {
	path, data string
}{
	{"ship/captain.txt", "The Captain"},
	{"ship/stowaway.txt", "Legatt"},
	{"story/name.txt", "The Secret Sharer"},
	{"story/author.txt", "Joseph Conrad"},
	{"multiline/test.txt", "bar\nfoo\n"},
	{"multiline/test_with_blank_lines.txt", "bar\nfoo\n\n\n"},
	{"empty/empty.txt", ""},
	{"empty/newline_only.txt", "\n"},
}

func getTestFiles() files {
	a := make(files, len(cases))
	for _, c := range cases {
		a[c.path] = []byte(c.data)
	}
	return a
}

func TestNewFiles(t *testing.T) {
	files := getTestFiles()
	assert.Len(t, files, len(cases), "Expected len() = %d, got %d", len(cases), len(files))

	for i, f := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, f.data, string(files.GetBytes(f.path)))
			assert.Equal(t, f.data, files.Get(f.path))
		})
	}
}

func TestFileGlob(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	matched := f.Glob("story/**")

	as.Len(matched, 2, "Should be two files in glob story/**")
	as.Equal("Joseph Conrad", matched.Get("story/author.txt"))
}

func TestToConfig(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()
	out := f.Glob("**/captain.txt").AsConfig()
	as.Equal("captain.txt: The Captain", out)

	out = f.Glob("ship/**").AsConfig()
	as.Equal("captain.txt: The Captain\nstowaway.txt: Legatt", out)
}

func TestToSecret(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out := f.Glob("ship/**").AsSecrets()
	as.Equal("captain.txt: VGhlIENhcHRhaW4=\nstowaway.txt: TGVnYXR0", out)
}

func TestLines(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out := f.Lines("multiline/test.txt")
	as.Len(out, 2)

	as.Equal("bar", out[0])
}

func TestBlankLines(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out := f.Lines("multiline/test_with_blank_lines.txt")
	as.Len(out, 4)

	as.Equal("bar", out[0])
	as.Empty(out[3])
}

func TestLinesEmptyFile(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out := f.Lines("empty/empty.txt")
	as.Empty(out)
}

func TestLinesNewlineOnlyFile(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out := f.Lines("empty/newline_only.txt")
	as.Len(out, 1)
	as.Empty(out[0])
}

func TestLinesMissingFile(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out := f.Lines("nonexistent.txt")
	as.Empty(out)
}
