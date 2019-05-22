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
	"encoding/json"
	"testing"
)

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
