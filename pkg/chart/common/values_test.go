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

package common

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadValues(t *testing.T) {
	doc := `# Test YAML parse
poet: "Coleridge"
title: "Rime of the Ancient Mariner"
stanza:
  - "at"
  - "length"
  - "did"
  - cross
  - an
  - Albatross

mariner:
  with: "crossbow"
  shot: "ALBATROSS"

water:
  water:
    where: "everywhere"
    nor: "any drop to drink"
`

	data, err := ReadValues([]byte(doc))
	require.NoErrorf(t, err, "Error parsing bytes")
	matchValues(t, data)

	tests := []string{`poet: "Coleridge"`, "# Just a comment", ""}

	for _, tt := range tests {
		data, err = ReadValues([]byte(tt))
		require.NoErrorf(t, err, "Error parsing bytes (%s)", tt)
		require.NotNilf(t, data, `YAML string "%s" gave a nil map`, tt)
	}
}

func TestReadValuesFile(t *testing.T) {
	data, err := ReadValuesFile("./testdata/coleridge.yaml")
	require.NoErrorf(t, err, "Error reading YAML file")
	matchValues(t, data)
}

func ExampleValues() {
	doc := `
title: "Moby Dick"
chapter:
  one:
    title: "Loomings"
  two:
    title: "The Carpet-Bag"
  three:
    title: "The Spouter Inn"
`
	d, err := ReadValues([]byte(doc))
	if err != nil {
		panic(err)
	}
	ch1, err := d.Table("chapter.one")
	if err != nil {
		panic("could not find chapter one")
	}
	fmt.Print(ch1["title"])
	// Output:
	// Loomings
}

func TestTable(t *testing.T) {
	doc := `
title: "Moby Dick"
chapter:
  one:
    title: "Loomings"
  two:
    title: "The Carpet-Bag"
  three:
    title: "The Spouter Inn"
`
	d, err := ReadValues([]byte(doc))
	require.NoErrorf(t, err, "Failed to parse the White Whale")

	_, err = d.Table("title")
	require.Error(t, err, "Title is not a table.")

	_, err = d.Table("chapter")
	require.NoErrorf(t, err, "Failed to get the chapter table: %v", d)

	v, err := d.Table("chapter.one")
	require.NoErrorf(t, err, "Failed to get chapter.one")
	assert.Equalf(t, "Loomings", v["title"], "Unexpected title: %s", v["title"])

	_, err = d.Table("chapter.three")
	require.NoErrorf(t, err, "Chapter three is missing: %v", d)

	_, err = d.Table("chapter.OneHundredThirtySix")
	assert.Error(t, err, "I think you mean 'Epilogue'")
}

func matchValues(t *testing.T, data map[string]any) {
	t.Helper()
	assert.Equalf(t, "Coleridge", data["poet"], "Unexpected poet: %s", data["poet"])

	o, err := ttpl("{{len .stanza}}", data)
	require.NoErrorf(t, err, "len stanza")
	assert.Equalf(t, "6", o, "Expected 6, got %s", o)

	o, err = ttpl("{{.mariner.shot}}", data)
	require.NoErrorf(t, err, ".mariner.shot")
	assert.Equal(t, "ALBATROSS", o, "Expected that mariner shot ALBATROSS")

	o, err = ttpl("{{.water.water.where}}", data)
	require.NoErrorf(t, err, ".water.water.where")
	assert.Equal(t, "everywhere", o, "Expected water water everywhere")
}

func ttpl(tpl string, v map[string]any) (string, error) {
	var b bytes.Buffer
	tt := template.Must(template.New("t").Parse(tpl))
	err := tt.Execute(&b, v)
	return b.String(), err
}

func TestPathValue(t *testing.T) {
	doc := `
title: "Moby Dick"
chapter:
  one:
    title: "Loomings"
  two:
    title: "The Carpet-Bag"
  three:
    title: "The Spouter Inn"
`
	d, err := ReadValues([]byte(doc))
	require.NoErrorf(t, err, "Failed to parse the White Whale")

	v, err := d.PathValue("chapter.one.title")
	require.NoErrorf(t, err, "Got error instead of title: %v", d)
	assert.Equalf(t, "Loomings", v, "No error but got wrong value for title: %v", d)
	_, err = d.PathValue("chapter.one.doesnotexist")
	require.Errorf(t, err, "Non-existent key should return error: %v", d)
	_, err = d.PathValue("chapter.doesnotexist.one")
	require.Errorf(t, err, "Non-existent key in middle of path should return error: %v", d)
	_, err = d.PathValue("")
	require.Error(t, err, "Asking for the value from an empty path should yield an error")
	v, err = d.PathValue("title")
	require.NoErrorf(t, err, "Failed to get title: %v", d)
	assert.Equalf(t, "Moby Dick", v, "Failed to return values for root key title: got %s\n%v", v, d)
}
