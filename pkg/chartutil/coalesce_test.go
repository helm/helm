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
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/chart"

	"github.com/stretchr/testify/assert"
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
	is := assert.New(t)
	c := loadChart(t, "testdata/moby")

	vals, err := ReadValues(testCoalesceValuesYaml)
	if err != nil {
		t.Fatal(err)
	}

	// taking a copy of the values before passing it
	// to CoalesceValues as argument, so that we can
	// use it for asserting later
	valsCopy := make(map[string]interface{}, len(vals))
	for key, value := range vals {
		valsCopy[key] = value
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

	// CoalesceValues should not mutate the passed arguments
	is.Equal(valsCopy, vals)
}

// Returns authoritative values
func getMainData() map[string]interface{} {
	return map[string]interface{}{
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
}

// Returns non-authoritative values
func getSecondaryData() map[string]interface{} {
	return map[string]interface{}{
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
}

// Tests the coalessing of getMainData() and getSecondaryData()
func testCoalescedData(t *testing.T, dst map[string]interface{}) {
	// What we expect is that anything in getMainData() overrides anything in
	// getSecondaryData(), but that otherwise the values are coalesced.
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

	if bo, ok := dst["boat"].(string); !ok {
		t.Fatalf("boat is the wrong type: %v", dst["boat"])
	} else if bo != "pequod" {
		t.Errorf("Expected boat string, got %v", dst["boat"])
	}
}

func TestCoalesceTables(t *testing.T) {
	dst := getMainData()
	src := getSecondaryData()

	CoalesceTables(dst, src)

	testCoalescedData(t, dst)
}

func TestCoalesceTablesUpdate(t *testing.T) {
	src := getMainData()
	dst := getSecondaryData()

	CoalesceTablesUpdate(dst, src)

	testCoalescedData(t, dst)
}

func TestCoalesceDep(t *testing.T) {
	src := map[string]interface{}{
		// global object should be transferred to subchart
		"global": map[string]interface{}{
			"IP":   "192.168.0.1",
			"port": 8080,
		},
		// subchart object should be coallesced with chart values and returned
		"subchart": getMainData(),
		// any other field should be ignored
		"other": map[string]interface{}{
			"type": "car",
		},
	}
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "subchart",
		},
		Values: getSecondaryData(),
	}
	subchart.Values["global"] = map[string]interface{}{
		"port":    80,
		"service": "users",
	}

	dst, err := CoalesceDep(subchart, src)
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := src["subchart"]; !ok {
		t.Fatal("subchart went away.")
	} else if dm, ok := d.(map[string]interface{}); !ok {
		t.Fatalf("subchart has now wrong type: %t", d)
	} else if reflect.ValueOf(dst).Pointer() != reflect.ValueOf(dm).Pointer() {
		t.Error("CoalesceDep must return subchart map.")
	}

	testCoalescedData(t, dst)

	glob, ok := dst["global"].(map[string]interface{})
	if !ok {
		t.Fatal("global went away.")
	}

	if glob["IP"].(string) != "192.168.0.1" {
		t.Errorf("Unexpected IP: %v", glob["IP"])
	}

	if glob["port"].(int) != 8080 {
		t.Errorf("Unexpected port: %v", glob["port"])
	}

	if glob["service"].(string) != "users" {
		t.Errorf("Unexpected service: %v", glob["service"])
	}

	if _, ok := dst["other"]; ok {
		t.Error("Unexpected field other.")
	}

	if _, ok := dst["type"]; ok {
		t.Error("Unexpected field type.")
	}
}

func TestCoalesceRoot(t *testing.T) {
	dst := getMainData()
	chart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "root",
		},
		Values: getSecondaryData(),
	}

	CoalesceRoot(chart, dst)

	testCoalescedData(t, dst)
}
