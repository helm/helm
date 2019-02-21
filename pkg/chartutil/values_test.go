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
	"encoding/json"
	"fmt"
	"testing"
	"text/template"

	kversion "k8s.io/apimachinery/pkg/version"

	"helm.sh/helm/pkg/chart"
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
		Metadata:  &chart.Metadata{Name: "test"},
		Templates: []*chart.File{},
		Values:    chartValues,
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
	}
	c.AddDependency(&chart.Chart{
		Metadata: &chart.Metadata{Name: "where"},
	})

	o := ReleaseOptions{
		Name:      "Seven Voyages",
		IsInstall: true,
	}

	caps := &Capabilities{
		APIVersions: DefaultVersionSet,
		KubeVersion: &kversion.Info{Major: "1"},
	}

	res, err := ToRenderValues(c, overideValues, o, caps)
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

	vals := res["Values"].(Values)
	if vals["name"] != "Haroun" {
		t.Errorf("Expected 'Haroun', got %q (%v)", vals["name"], vals)
	}
	where := vals["where"].(map[string]interface{})
	expects := map[string]string{
		"city":  "Baghdad",
		"date":  "809 CE",
		"title": "caliph",
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

func TestReadSchemaFile(t *testing.T) {
	data, err := ReadSchemaFile("./testdata/test-values.schema.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	matchSchema(t, data)
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
	err := tt.Execute(&b, v)
	return b.String(), err
}

// ref: http://www.yaml.org/spec/1.2/spec.html#id2803362
var testCoalesceValuesYaml = []byte(`
top: yup
bottom: null
right: Null
left: NULL
front: ~
back: ""

global:
  name: Ishmael
  subject: Queequeg
  nested:
    boat: true

pequod:
  global:
    name: Stinky
    harpooner: Tashtego
    nested:
      boat: false
      sail: true
  ahab:
    scope: whale
`)

func TestCoalesceValues(t *testing.T) {
	c := loadChart(t, "testdata/moby")

	vals, err := ReadValues(testCoalesceValuesYaml)
	if err != nil {
		t.Fatal(err)
	}

	v, err := CoalesceValues(c, vals)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := json.MarshalIndent(v, "", "  ")
	t.Logf("Coalesced Values: %s", string(j))

	tests := []struct {
		tpl    string
		expect string
	}{
		{"{{.top}}", "yup"},
		{"{{.back}}", ""},
		{"{{.name}}", "moby"},
		{"{{.global.name}}", "Ishmael"},
		{"{{.global.subject}}", "Queequeg"},
		{"{{.global.harpooner}}", "<no value>"},
		{"{{.pequod.name}}", "pequod"},
		{"{{.pequod.ahab.name}}", "ahab"},
		{"{{.pequod.ahab.scope}}", "whale"},
		{"{{.pequod.ahab.global.name}}", "Ishmael"},
		{"{{.pequod.ahab.global.subject}}", "Queequeg"},
		{"{{.pequod.ahab.global.harpooner}}", "Tashtego"},
		{"{{.pequod.global.name}}", "Ishmael"},
		{"{{.pequod.global.subject}}", "Queequeg"},
		{"{{.spouter.global.name}}", "Ishmael"},
		{"{{.spouter.global.harpooner}}", "<no value>"},

		{"{{.global.nested.boat}}", "true"},
		{"{{.pequod.global.nested.boat}}", "true"},
		{"{{.spouter.global.nested.boat}}", "true"},
		{"{{.pequod.global.nested.sail}}", "true"},
		{"{{.spouter.global.nested.sail}}", "<no value>"},
	}

	for _, tt := range tests {
		if o, err := ttpl(tt.tpl, v); err != nil || o != tt.expect {
			t.Errorf("Expected %q to expand to %q, got %q", tt.tpl, tt.expect, o)
		}
	}

	nullKeys := []string{"bottom", "right", "left", "front"}
	for _, nullKey := range nullKeys {
		if _, ok := v[nullKey]; ok {
			t.Errorf("Expected key %q to be removed, still present", nullKey)
		}
	}
}

func TestCoalesceTables(t *testing.T) {
	dst := map[string]interface{}{
		"name": "Ishmael",
		"address": map[string]interface{}{
			"street": "123 Spouter Inn Ct.",
			"city":   "Nantucket",
		},
		"details": map[string]interface{}{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
	}
	src := map[string]interface{}{
		"occupation": "whaler",
		"address": map[string]interface{}{
			"state":  "MA",
			"street": "234 Spouter Inn Ct.",
		},
		"details": "empty",
		"boat": map[string]interface{}{
			"mast": true,
		},
	}

	// What we expect is that anything in dst overrides anything in src, but that
	// otherwise the values are coalesced.
	CoalesceTables(dst, src)

	if dst["name"] != "Ishmael" {
		t.Errorf("Unexpected name: %s", dst["name"])
	}
	if dst["occupation"] != "whaler" {
		t.Errorf("Unexpected occupation: %s", dst["occupation"])
	}

	addr, ok := dst["address"].(map[string]interface{})
	if !ok {
		t.Fatal("Address went away.")
	}

	if addr["street"].(string) != "123 Spouter Inn Ct." {
		t.Errorf("Unexpected address: %v", addr["street"])
	}

	if addr["city"].(string) != "Nantucket" {
		t.Errorf("Unexpected city: %v", addr["city"])
	}

	if addr["state"].(string) != "MA" {
		t.Errorf("Unexpected state: %v", addr["state"])
	}

	if det, ok := dst["details"].(map[string]interface{}); !ok {
		t.Fatalf("Details is the wrong type: %v", dst["details"])
	} else if _, ok := det["friends"]; !ok {
		t.Error("Could not find your friends. Maybe you don't have any. :-(")
	}

	if dst["boat"].(string) != "pequod" {
		t.Errorf("Expected boat string, got %v", dst["boat"])
	}
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

func TestReadSchema(t *testing.T) {
	schemaTest := `# Test YAML parse
title: Values
type: object
properties:
  firstname:
    description: First name
    type: string
  lastname:
    type: string
  likes-coffee:
    type: boolean
  age:
    description: Age
    type: integer
    minimum: 0
  employment-info:
    type: object
    properties:
      salary:
        type: float
        minimum: 0
      title:
        type: string
    required:
      - salary
  addresses:
    description: List of addresses
    type: list[object]
    properties:
      city:
        type: string
      street:
        type: string
      number:
        type: number
  phone-numbers:
    type: list[string]
required:
  - firstname
  - lastname
  - addresses
  - employment-info
`
	data, err := ReadSchema([]byte(schemaTest))
	if err != nil {
		t.Fatalf("Error parsing bytes: %s", err)
	}
	matchSchema(t, data)
}

func matchSchema(t *testing.T, data Schema) {
	if data.Title != "Values" {
		t.Errorf("Expected .title to be 'Values', got '%s'", data.Title)
	}

	if data.Type != "object" {
		t.Errorf("Expected .type to be 'object', got '%s'", data.Type)
	}
	expectedRequired := []string{
		"firstname",
		"lastname",
		"addresses",
		"employment-info",
	}
	if len(data.Required) != 4 {
		t.Errorf("Expected length of .required to be 4, got %d", len(data.Required))
	}

	if !assertEqualSlices(data.Required, expectedRequired) {
		t.Errorf("Expected .required to be %v, got %v", expectedRequired, data.Required)
	}

	var ok bool
	var firstname *Schema
	if firstname, ok = data.Properties["firstname"]; !ok {
		t.Errorf("Expected property '.properties.firstname' is missing")
	}
	if firstname.Description != "First name" {
		t.Errorf("Expected .properties.firstname.description to be 'First name', got '%s'", firstname.Description)
	}
	if firstname.Type != "string" {
		t.Errorf("Expected .properties.firstname.type to be 'string', got '%s'", firstname.Type)
	}

	var lastname *Schema
	if lastname, ok = data.Properties["lastname"]; !ok {
		t.Errorf("Expected property '.properties.lastname' is missing")
	}
	if lastname.Type != "string" {
		t.Errorf("Expected .properties.lastname.type to be 'string', got '%s'", lastname.Type)
	}

	var likesCoffee *Schema
	if likesCoffee, ok = data.Properties["likes-coffee"]; !ok {
		t.Errorf("Expected property '.properties.likes-coffee' is missing")
	}
	if likesCoffee.Type != "boolean" {
		t.Errorf("Expected .properties.likes-coffee.type to be 'boolean', got '%s'", likesCoffee.Type)
	}

	var age *Schema
	if age, ok = data.Properties["age"]; !ok {
		t.Errorf("Expected property '.properties.age' is missing")
	}
	if age.Description != "Age" {
		t.Errorf("Expected .properties.age.description to be 'Age', got '%s'", age.Description)
	}
	if age.Type != "integer" {
		t.Errorf("Expected .properties.age.type to be 'string', got '%s'", age.Type)
	}
	if age.Minimum != 0 {
		t.Errorf("Expected .properties.age.minimum to be 0, got %d", age.Minimum)
	}

	var employmentInfo *Schema
	if employmentInfo, ok = data.Properties["employment-info"]; !ok {
		t.Errorf("Expected property '.properties.employment-info' is missing")
	}
	if employmentInfo.Type != "object" {
		t.Errorf("Expected .properties.employment-info.type to be 'object', got '%s'", employmentInfo.Type)
	}
	if len(employmentInfo.Required) != 1 {
		t.Errorf("Expected length of .properties.employment-info.required to be 1, got %d", len(employmentInfo.Required))
	}
	if !assertEqualSlices(employmentInfo.Required, []string{"salary"}) {
		t.Errorf("Expected .properties.employment-info.required to be %v, got %v", []string{"salary"}, data.Required)
	}

	var salary *Schema
	if salary, ok = employmentInfo.Properties["salary"]; !ok {
		t.Errorf("Expected property '.properties.employment-info.properties.salary' is missing")
	}
	if salary.Type != "float" {
		t.Errorf("Expected .properties.employment-info.properties.salary.type to be 'float', got '%s'", salary.Type)
	}
	if salary.Minimum != 0 {
		t.Errorf("Expected .properties.employment-info.properties.salary.minimum to be 0, got %d", salary.Minimum)
	}

	var title *Schema
	if title, ok = employmentInfo.Properties["title"]; !ok {
		t.Errorf("Expected property '.properties.employment-info.properties.title' is missing")
	}
	if title.Type != "string" {
		t.Errorf("Expected .properties.employment-info.properties.title.type to be 'string', got '%s'", title.Type)
	}

	var addresses *Schema
	if addresses, ok = data.Properties["addresses"]; !ok {
		t.Errorf("Expected property '.properties.addresses' is missing")
	}
	if addresses.Type != "list[object]" {
		t.Errorf("Expected .properties.addresses.type to be 'list[object]', got '%s'", addresses.Type)
	}
	if addresses.Description != "List of addresses" {
		t.Errorf("Expected .properties.addresses.description to be 'List of addresses', got '%s'", addresses.Description)
	}

	var city *Schema
	if city, ok = addresses.Properties["city"]; !ok {
		t.Errorf("Expected property '.properties.addresses.properties.city' is missing")
	}
	if city.Type != "string" {
		t.Errorf("Expected .properties.addresses.properties.city.type to be 'string', got '%s'", city.Type)
	}

	var street *Schema
	if street, ok = addresses.Properties["street"]; !ok {
		t.Errorf("Expected property '.properties.addresses.properties.street' is missing")
	}
	if street.Type != "string" {
		t.Errorf("Expected .properties.addresses.properties.street.type to be 'string', got '%s'", street.Type)
	}

	var number *Schema
	if number, ok = addresses.Properties["number"]; !ok {
		t.Errorf("Expected property '.properties.addresses.properties.number' is missing")
	}
	if number.Type != "number" {
		t.Errorf("Expected .properties.addresses.properties.number.type to be 'number', got '%s'", number.Type)
	}

	var phoneNumbers *Schema
	if phoneNumbers, ok = data.Properties["phone-numbers"]; !ok {
		t.Errorf("Expected property '.properties.phone-numbers' is missing")
	}
	if phoneNumbers.Type != "list[string]" {
		t.Errorf("Expected .properties.phone-numbers.type to be 'list[object]', got '%s'", addresses.Type)
	}
}

func assertEqualSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
