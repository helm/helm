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
	"testing"

	"github.com/stretchr/testify/assert"
)

const NonExistingFileName = "no_such_file.txt"

var cases = []struct {
	path, data string
}{
	{"ship/captain.txt", "The Captain"},
	{"ship/stowaway.txt", "Legatt"},
	{"story/name.txt", "The Secret Sharer"},
	{"story/author.txt", "Joseph Conrad"},
	{"multiline/test.txt", "bar\nfoo"},
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
	if len(files) != len(cases) {
		t.Errorf("Expected len() = %d, got %d", len(cases), len(files))
	}

	for i, f := range cases {
		gotBytes, err := files.GetBytes(f.path)
		got := string(gotBytes)
		if err != nil || got != f.data {
			t.Errorf("%d: expected %q, got %q", i, f.data, got)
		}

		gotBytes, err = files.GetBytes(f.path)
		got = string(gotBytes)
		if err != nil || got != f.data {
			t.Errorf("%d: expected %q, got %q", i, f.data, got)
		}
	}
}

func TestGetNonExistingFile(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	content, err := f.Get(NonExistingFileName)
	as.Empty(content)
	as.Error(err, "not included")
}

func TestFileGlob(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	matched := f.Glob("story/**")

	as.Len(matched, 2, "Should be two files in glob story/**")

	content, err := matched.Get("story/author.txt")
	as.Equal("Joseph Conrad", content)
	as.NoError(err)
}

func TestToConfig(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()
	out, err := f.Glob("**/captain.txt").AsConfig()
	as.Equal("captain.txt: The Captain", out)
	as.NoError(err)

	out, err = f.Glob("ship/**").AsConfig()
	as.Equal("captain.txt: The Captain\nstowaway.txt: Legatt", out)
	as.NoError(err)

	out, err = f.Glob(NonExistingFileName).AsConfig()
	as.Empty(out)
	as.Error(err, "must pass files")
}

func TestToSecret(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out, err := f.Glob("ship/**").AsSecrets()
	as.Equal("captain.txt: VGhlIENhcHRhaW4=\nstowaway.txt: TGVnYXR0", out)
	as.NoError(err)

	out, err = f.Glob(NonExistingFileName).AsSecrets()
	as.Empty(out)
	as.Errorf(err, "must pass files")
}

func TestLines(t *testing.T) {
	as := assert.New(t)

	f := getTestFiles()

	out, err := f.Lines("multiline/test.txt")
	as.Len(out, 2)
	as.Equal("bar", out[0])
	as.NoError(err)

	out, err = f.Lines(NonExistingFileName)
	as.Nil(out)
	as.Error(err, "must pass files")
}
