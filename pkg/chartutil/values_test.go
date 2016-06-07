package chartutil

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
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
	if err != nil {
		t.Fatalf("Error parsing bytes: %s", err)
	}
	matchValues(t, data)
}

func TestReadValuesFile(t *testing.T) {
	data, err := ReadValuesFile("./testdata/coleridge.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
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
	if err != nil {
		t.Fatalf("Failed to parse the White Whale: %s", err)
	}

	if _, err := d.Table("title"); err == nil {
		t.Fatalf("Title is not a table.")
	}

	if _, err := d.Table("chapter"); err != nil {
		t.Fatalf("Failed to get the chapter table: %s\n%v", err, d)
	}

	if v, err := d.Table("chapter.one"); err != nil {
		t.Errorf("Failed to get chapter.one: %s", err)
	} else if v["title"] != "Loomings" {
		t.Errorf("Unexpected title: %s", v["title"])
	}

	if _, err := d.Table("chapter.three"); err != nil {
		t.Errorf("Chapter three is missing: %s\n%v", err, d)
	}

	if _, err := d.Table("chapter.OneHundredThirtySix"); err == nil {
		t.Errorf("I think you mean 'Epilogue'")
	}
}

func matchValues(t *testing.T, data map[string]interface{}) {
	if data["poet"] != "Coleridge" {
		t.Errorf("Unexpected poet: %s", data["poet"])
	}

	if o, err := ttpl("{{len .stanza}}", data); err != nil {
		t.Errorf("len stanza: %s", err)
	} else if o != "6" {
		t.Errorf("Expected 6, got %s", o)
	}

	if o, err := ttpl("{{.mariner.shot}}", data); err != nil {
		t.Errorf(".mariner.shot: %s", err)
	} else if o != "ALBATROSS" {
		t.Errorf("Expected that mariner shot ALBATROSS")
	}

	if o, err := ttpl("{{.water.water.where}}", data); err != nil {
		t.Errorf(".water.water.where: %s", err)
	} else if o != "everywhere" {
		t.Errorf("Expected water water everywhere")
	}
}

func ttpl(tpl string, v map[string]interface{}) (string, error) {
	var b bytes.Buffer
	tt := template.Must(template.New("t").Parse(tpl))
	if err := tt.Execute(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}
