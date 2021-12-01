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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chart"
)

// ref: http://www.yaml.org/spec/1.2/spec.html#id2803362
var testCoalesceValuesYaml = []byte(`
top: yup
bottom: null
right: Null
left: NULL
front: ~
back: ""
nested:
  boat: null

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
    boat: null
    nested:
      foo: true
      bar: null
`)

func withDeps(c *chart.Chart, deps ...*chart.Chart) *chart.Chart {
	c.AddDependency(deps...)
	return c
}

func TestCoalesceValues(t *testing.T) {
	is := assert.New(t)

	c := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "moby"},
		Values: map[string]interface{}{
			"back":     "exists",
			"bottom":   "exists",
			"front":    "exists",
			"left":     "exists",
			"name":     "moby",
			"nested":   map[string]interface{}{"boat": true},
			"override": "bad",
			"right":    "exists",
			"scope":    "moby",
			"top":      "nope",
			"global": map[string]interface{}{
				"nested2": map[string]interface{}{"l0": "moby"},
			},
		},
	},
		withDeps(&chart.Chart{
			Metadata: &chart.Metadata{Name: "pequod"},
			Values: map[string]interface{}{
				"name":  "pequod",
				"scope": "pequod",
				"global": map[string]interface{}{
					"nested2": map[string]interface{}{"l1": "pequod"},
				},
			},
		},
			&chart.Chart{
				Metadata: &chart.Metadata{Name: "ahab"},
				Values: map[string]interface{}{
					"global": map[string]interface{}{
						"nested":  map[string]interface{}{"foo": "bar"},
						"nested2": map[string]interface{}{"l2": "ahab"},
					},
					"scope":  "ahab",
					"name":   "ahab",
					"boat":   true,
					"nested": map[string]interface{}{"foo": false, "bar": true},
				},
			},
		),
		&chart.Chart{
			Metadata: &chart.Metadata{Name: "spouter"},
			Values: map[string]interface{}{
				"scope": "spouter",
				"global": map[string]interface{}{
					"nested2": map[string]interface{}{"l1": "spouter"},
				},
			},
		},
	)

	vals, err := ReadValues(testCoalesceValuesYaml)
	if err != nil {
		t.Fatal(err)
	}

	// taking a copy of the values before passing it
	// to CoalesceValues as argument, so that we can
	// use it for asserting later
	valsCopy := make(Values, len(vals))
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
		{"{{.pequod.ahab.nested.foo}}", "true"},
		{"{{.pequod.ahab.global.name}}", "Ishmael"},
		{"{{.pequod.ahab.global.nested.foo}}", "bar"},
		{"{{.pequod.ahab.global.subject}}", "Queequeg"},
		{"{{.pequod.ahab.global.harpooner}}", "Tashtego"},
		{"{{.pequod.global.name}}", "Ishmael"},
		{"{{.pequod.global.nested.foo}}", "<no value>"},
		{"{{.pequod.global.subject}}", "Queequeg"},
		{"{{.spouter.global.name}}", "Ishmael"},
		{"{{.spouter.global.harpooner}}", "<no value>"},

		{"{{.global.nested.boat}}", "true"},
		{"{{.pequod.global.nested.boat}}", "true"},
		{"{{.spouter.global.nested.boat}}", "true"},
		{"{{.pequod.global.nested.sail}}", "true"},
		{"{{.spouter.global.nested.sail}}", "<no value>"},

		{"{{.global.nested2.l0}}", "moby"},
		{"{{.global.nested2.l1}}", "<no value>"},
		{"{{.global.nested2.l2}}", "<no value>"},
		{"{{.pequod.global.nested2.l0}}", "moby"},
		{"{{.pequod.global.nested2.l1}}", "pequod"},
		{"{{.pequod.global.nested2.l2}}", "<no value>"},
		{"{{.pequod.ahab.global.nested2.l0}}", "moby"},
		{"{{.pequod.ahab.global.nested2.l1}}", "pequod"},
		{"{{.pequod.ahab.global.nested2.l2}}", "ahab"},
		{"{{.spouter.global.nested2.l0}}", "moby"},
		{"{{.spouter.global.nested2.l1}}", "spouter"},
		{"{{.spouter.global.nested2.l2}}", "<no value>"},
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

	if _, ok := v["nested"].(map[string]interface{})["boat"]; ok {
		t.Error("Expected nested boat key to be removed, still present")
	}

	subchart := v["pequod"].(map[string]interface{})["ahab"].(map[string]interface{})
	if _, ok := subchart["boat"]; ok {
		t.Error("Expected subchart boat key to be removed, still present")
	}

	if _, ok := subchart["nested"].(map[string]interface{})["bar"]; ok {
		t.Error("Expected subchart nested bar key to be removed, still present")
	}

	// CoalesceValues should not mutate the passed arguments
	is.Equal(valsCopy, vals)
}

func TestCoalesceTables(t *testing.T) {
	dst := map[string]interface{}{
		"name": "Ishmael",
		"address": map[string]interface{}{
			"street":  "123 Spouter Inn Ct.",
			"city":    "Nantucket",
			"country": nil,
		},
		"details": map[string]interface{}{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
		"hole": nil,
	}
	src := map[string]interface{}{
		"occupation": "whaler",
		"address": map[string]interface{}{
			"state":   "MA",
			"street":  "234 Spouter Inn Ct.",
			"country": "US",
		},
		"details": "empty",
		"boat": map[string]interface{}{
			"mast": true,
		},
		"hole": "black",
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

	if _, ok = addr["country"]; ok {
		t.Error("The country is not left out.")
	}

	if det, ok := dst["details"].(map[string]interface{}); !ok {
		t.Fatalf("Details is the wrong type: %v", dst["details"])
	} else if _, ok := det["friends"]; !ok {
		t.Error("Could not find your friends. Maybe you don't have any. :-(")
	}

	if dst["boat"].(string) != "pequod" {
		t.Errorf("Expected boat string, got %v", dst["boat"])
	}

	if _, ok = dst["hole"]; ok {
		t.Error("The hole still exists.")
	}

	dst2 := map[string]interface{}{
		"name": "Ishmael",
		"address": map[string]interface{}{
			"street":  "123 Spouter Inn Ct.",
			"city":    "Nantucket",
			"country": "US",
		},
		"details": map[string]interface{}{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
		"hole": "black",
	}

	// What we expect is that anything in dst should have all values set,
	// this happens when the --reuse-values flag is set but the chart has no modifications yet
	CoalesceTables(dst2, nil)

	if dst2["name"] != "Ishmael" {
		t.Errorf("Unexpected name: %s", dst2["name"])
	}

	addr2, ok := dst2["address"].(map[string]interface{})
	if !ok {
		t.Fatal("Address went away.")
	}

	if addr2["street"].(string) != "123 Spouter Inn Ct." {
		t.Errorf("Unexpected address: %v", addr2["street"])
	}

	if addr2["city"].(string) != "Nantucket" {
		t.Errorf("Unexpected city: %v", addr2["city"])
	}

	if addr2["country"].(string) != "US" {
		t.Errorf("Unexpected Country: %v", addr2["country"])
	}

	if det2, ok := dst2["details"].(map[string]interface{}); !ok {
		t.Fatalf("Details is the wrong type: %v", dst2["details"])
	} else if _, ok := det2["friends"]; !ok {
		t.Error("Could not find your friends. Maybe you don't have any. :-(")
	}

	if dst2["boat"].(string) != "pequod" {
		t.Errorf("Expected boat string, got %v", dst2["boat"])
	}

	if dst2["hole"].(string) != "black" {
		t.Errorf("Expected hole string, got %v", dst2["boat"])
	}
}

func TestCoalesceValuesWarnings(t *testing.T) {

	c := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "level1"},
		Values: map[string]interface{}{
			"name": "moby",
		},
	},
		withDeps(&chart.Chart{
			Metadata: &chart.Metadata{Name: "level2"},
			Values: map[string]interface{}{
				"name": "pequod",
			},
		},
			&chart.Chart{
				Metadata: &chart.Metadata{Name: "level3"},
				Values: map[string]interface{}{
					"name": "ahab",
					"boat": true,
					"spear": map[string]interface{}{
						"tip": true,
						"sail": map[string]interface{}{
							"cotton": true,
						},
					},
				},
			},
		),
	)

	vals := map[string]interface{}{
		"level2": map[string]interface{}{
			"level3": map[string]interface{}{
				"boat": map[string]interface{}{"mast": true},
				"spear": map[string]interface{}{
					"tip": map[string]interface{}{
						"sharp": true,
					},
					"sail": true,
				},
			},
		},
	}

	warnings := make([]string, 0)
	printf := func(format string, v ...interface{}) {
		t.Logf(format, v...)
		warnings = append(warnings, fmt.Sprintf(format, v...))
	}

	_, err := coalesce(printf, c, vals, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("vals: %v", vals)
	assert.Contains(t, warnings, "warning: skipped value for level1.level2.level3.boat: Not a table.")
	assert.Contains(t, warnings, "warning: destination for level1.level2.level3.spear.tip is a table. Ignoring non-table value (true)")
	assert.Contains(t, warnings, "warning: cannot overwrite table with non table for level1.level2.level3.spear.sail (map[cotton:true])")

}

func TestConcatPrefix(t *testing.T) {
	assert.Equal(t, "b", concatPrefix("", "b"))
	assert.Equal(t, "a.b", concatPrefix("a", "b"))
}
