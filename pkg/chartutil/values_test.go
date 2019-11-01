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

package chartutil

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"helm.sh/helm/v3/pkg/chart"
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

	tests := []string{`poet: "Coleridge"`, "# Just a comment", ""}

	for _, tt := range tests {
		data, err = ReadValues([]byte(tt))
		if err != nil {
			t.Fatalf("Error parsing bytes (%s): %s", tt, err)
		}
		if data == nil {
			t.Errorf(`YAML string "%s" gave a nil map`, tt)
		}
	}
}

func TestToRenderValues(t *testing.T) {

	chartValues := map[string]interface{}{
		"name": "al Rashid",
		"where": map[string]interface{}{
			"city":  "Basrah",
			"title": "caliph",
		},
	}

	overideValues := map[string]interface{}{
		"name": "Haroun",
		"where": map[string]interface{}{
			"city": "Baghdad",
			"date": "809 CE",
		},
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
		Values:   chartValues,
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
	}
	c.AddDependency(&chart.Chart{
		Metadata: &chart.Metadata{Name: "where"},
	})

	o := ReleaseOptions{
		Name:      "Seven Voyages",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}

	res, err := ToRenderValues(c, overideValues, o, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that the top-level values are all set.
	if name := res["Chart"].(*chart.Metadata).Name; name != "test" {
		t.Errorf("Expected chart name 'test', got %q", name)
	}
	relmap := res["Release"].(map[string]interface{})
	if name := relmap["Name"]; name.(string) != "Seven Voyages" {
		t.Errorf("Expected release name 'Seven Voyages', got %q", name)
	}
	if namespace := relmap["Namespace"]; namespace.(string) != "default" {
		t.Errorf("Expected namespace 'default', got %q", namespace)
	}
	if revision := relmap["Revision"]; revision.(int) != 1 {
		t.Errorf("Expected revision '1', got %d", revision)
	}
	if relmap["IsUpgrade"].(bool) {
		t.Error("Expected upgrade to be false.")
	}
	if !relmap["IsInstall"].(bool) {
		t.Errorf("Expected install to be true.")
	}
	if !res["Capabilities"].(*Capabilities).APIVersions.Has("v1") {
		t.Error("Expected Capabilities to have v1 as an API")
	}
	if res["Capabilities"].(*Capabilities).KubeVersion.Major != "1" {
		t.Error("Expected Capabilities to have a Kube version")
	}

	vals := res["Values"].(map[string]interface{})
	if vals["name"] != "Haroun" {
		t.Errorf("Expected 'Haroun', got %q (%v)", vals["name"], vals)
	}
	where := vals["where"].(map[string]interface{})
	expects := map[string]string{
		"city": "Baghdad",
		"date": "809 CE",
		// ToRenderValues no longer coallesce chart values
		// "title": "caliph",
	}
	for field, expect := range expects {
		if got := where[field]; got != expect {
			t.Errorf("Expected %q, got %q (%v)", expect, got, where)
		}
	}
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
	var d Values
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
	var d Values
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
	var d Values
	d, err := ReadValues([]byte(doc))
	if err != nil {
		t.Fatalf("Failed to parse the White Whale: %s", err)
	}

	if v, err := d.PathValue("chapter.one.title"); err != nil {
		t.Errorf("Got error instead of title: %s\n%v", err, d)
	} else if v != "Loomings" {
		t.Errorf("No error but got wrong value for title: %s\n%v", err, d)
	}
	if _, err := d.PathValue("chapter.one.doesntexist"); err == nil {
		t.Errorf("Non-existent key should return error: %s\n%v", err, d)
	}
	if _, err := d.PathValue("chapter.doesntexist.one"); err == nil {
		t.Errorf("Non-existent key in middle of path should return error: %s\n%v", err, d)
	}
	if _, err := d.PathValue(""); err == nil {
		t.Error("Asking for the value from an empty path should yield an error")
	}
	if v, err := d.PathValue("title"); err == nil {
		if v != "Moby Dick" {
			t.Errorf("Failed to return values for root key title")
		}
	}
}
