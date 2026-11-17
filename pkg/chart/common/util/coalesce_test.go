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

package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
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
  boat: null
  global:
    name: Stinky
    harpooner: Tashtego
    nested:
      boat: false
      sail: true
      foo2: null
  ahab:
    scope: whale
    boat: null
    nested:
      foo: true
      boat: null
    object: null
`)

func withDeps(c *chart.Chart, deps ...*chart.Chart) *chart.Chart {
	c.AddDependency(deps...)
	return c
}

func TestCoalesceValues(t *testing.T) {
	is := assert.New(t)

	c := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "moby"},
		Values: map[string]any{
			"back":     "exists",
			"bottom":   "exists",
			"front":    "exists",
			"left":     "exists",
			"name":     "moby",
			"nested":   map[string]any{"boat": true},
			"override": "bad",
			"right":    "exists",
			"scope":    "moby",
			"top":      "nope",
			"global": map[string]any{
				"nested2": map[string]any{"l0": "moby"},
			},
			"pequod": map[string]any{
				"boat": "maybe",
				"ahab": map[string]any{
					"boat":   "maybe",
					"nested": map[string]any{"boat": "maybe"},
				},
			},
		},
	},
		withDeps(&chart.Chart{
			Metadata: &chart.Metadata{Name: "pequod"},
			Values: map[string]any{
				"name":  "pequod",
				"scope": "pequod",
				"global": map[string]any{
					"nested2": map[string]any{"l1": "pequod"},
				},
				"boat": false,
				"ahab": map[string]any{
					"boat":   false,
					"nested": map[string]any{"boat": false},
				},
			},
		},
			&chart.Chart{
				Metadata: &chart.Metadata{Name: "ahab"},
				Values: map[string]any{
					"global": map[string]any{
						"nested":  map[string]any{"foo": "bar", "foo2": "bar2"},
						"nested2": map[string]any{"l2": "ahab"},
					},
					"scope":  "ahab",
					"name":   "ahab",
					"boat":   true,
					"nested": map[string]any{"foo": false, "boat": true},
					"object": map[string]any{"foo": "bar"},
				},
			},
		),
		&chart.Chart{
			Metadata: &chart.Metadata{Name: "spouter"},
			Values: map[string]any{
				"scope": "spouter",
				"global": map[string]any{
					"nested2": map[string]any{"l1": "spouter"},
				},
			},
		},
	)

	vals, err := common.ReadValues(testCoalesceValuesYaml)
	require.NoError(t, err)

	// taking a copy of the values before passing it
	// to CoalesceValues as argument, so that we can
	// use it for asserting later
	valsCopy := make(common.Values, len(vals))
	maps.Copy(valsCopy, vals)

	v, err := CoalesceValues(c, vals)
	require.NoError(t, err)
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
		{"{{.pequod.ahab.global.nested.foo2}}", "<no value>"},
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
		_, ok := v[nullKey]
		assert.Falsef(t, ok, "Expected key %q to be removed, still present", nullKey)
	}

	_, ok := v["nested"].(map[string]any)["boat"]
	assert.False(t, ok, "Expected nested boat key to be removed, still present")

	subchart := v["pequod"].(map[string]any)
	_, ok = subchart["boat"]
	assert.False(t, ok, "Expected subchart boat key to be removed, still present")

	subsubchart := subchart["ahab"].(map[string]any)
	_, ok = subsubchart["boat"]
	assert.False(t, ok, "Expected sub-subchart ahab boat key to be removed, still present")

	_, ok = subsubchart["nested"].(map[string]any)["boat"]
	assert.False(t, ok, "Expected sub-subchart nested boat key to be removed, still present")

	_, ok = subsubchart["object"]
	assert.False(t, ok, "Expected sub-subchart object map to be removed, still present")

	// CoalesceValues should not mutate the passed arguments
	is.Equal(valsCopy, vals)
}

func ttpl(tpl string, v map[string]any) (string, error) {
	var b bytes.Buffer
	tt := template.Must(template.New("t").Parse(tpl))
	err := tt.Execute(&b, v)
	return b.String(), err
}

func TestMergeValues(t *testing.T) {
	is := assert.New(t)

	c := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "moby"},
		Values: map[string]any{
			"back":     "exists",
			"bottom":   "exists",
			"front":    "exists",
			"left":     "exists",
			"name":     "moby",
			"nested":   map[string]any{"boat": true},
			"override": "bad",
			"right":    "exists",
			"scope":    "moby",
			"top":      "nope",
			"global": map[string]any{
				"nested2": map[string]any{"l0": "moby"},
			},
		},
	},
		withDeps(&chart.Chart{
			Metadata: &chart.Metadata{Name: "pequod"},
			Values: map[string]any{
				"name":  "pequod",
				"scope": "pequod",
				"global": map[string]any{
					"nested2": map[string]any{"l1": "pequod"},
				},
			},
		},
			&chart.Chart{
				Metadata: &chart.Metadata{Name: "ahab"},
				Values: map[string]any{
					"global": map[string]any{
						"nested":  map[string]any{"foo": "bar"},
						"nested2": map[string]any{"l2": "ahab"},
					},
					"scope":  "ahab",
					"name":   "ahab",
					"boat":   true,
					"nested": map[string]any{"foo": false, "bar": true},
				},
			},
		),
		&chart.Chart{
			Metadata: &chart.Metadata{Name: "spouter"},
			Values: map[string]any{
				"scope": "spouter",
				"global": map[string]any{
					"nested2": map[string]any{"l1": "spouter"},
				},
			},
		},
	)

	vals, err := common.ReadValues(testCoalesceValuesYaml)
	require.NoError(t, err)

	// taking a copy of the values before passing it
	// to MergeValues as argument, so that we can
	// use it for asserting later
	valsCopy := make(common.Values, len(vals))
	maps.Copy(valsCopy, vals)

	v, err := MergeValues(c, vals)
	require.NoError(t, err)
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

	// nullKeys is different from coalescing. Here the null/nil values are not
	// removed.
	nullKeys := []string{"bottom", "right", "left", "front"}
	for _, nullKey := range nullKeys {
		vv, ok := v[nullKey]
		assert.Truef(t, ok, "Expected key %q to be present but it was removed", nullKey)
		assert.Nilf(t, vv, "Expected key %q to be null but it has a value of %v", nullKey, vv)
	}

	_, ok := v["nested"].(map[string]any)["boat"]
	assert.True(t, ok, "Expected nested boat key to be present but it was removed")

	subchart := v["pequod"].(map[string]any)["ahab"].(map[string]any)
	assert.Contains(t, subchart, "boat", "Expected subchart boat key to be present but it was removed")

	_, ok = subchart["nested"].(map[string]any)["bar"]
	assert.True(t, ok, "Expected subchart nested bar key to be present but it was removed")

	// CoalesceValues should not mutate the passed arguments
	is.Equal(valsCopy, vals)
}

func TestCoalesceTables(t *testing.T) {
	dst := map[string]any{
		"name": "Ishmael",
		"address": map[string]any{
			"street":  "123 Spouter Inn Ct.",
			"city":    "Nantucket",
			"country": nil,
		},
		"details": map[string]any{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
		"hole": nil,
	}
	src := map[string]any{
		"occupation": "whaler",
		"address": map[string]any{
			"state":   "MA",
			"street":  "234 Spouter Inn Ct.",
			"country": "US",
		},
		"details": "empty",
		"boat": map[string]any{
			"mast": true,
		},
		"hole": "black",
	}

	// What we expect is that anything in dst overrides anything in src, but that
	// otherwise the values are coalesced.
	CoalesceTables(dst, src)

	assert.Equal(t, "Ishmael", dst["name"], "Unexpected name: %s", dst["name"])
	assert.Equal(t, "whaler", dst["occupation"], "Unexpected occupation: %s", dst["occupation"])

	addr, ok := dst["address"].(map[string]any)
	require.True(t, ok, "Address went away.")
	assert.Equal(t, "123 Spouter Inn Ct.", addr["street"].(string), "Unexpected address: %v", addr["street"])
	assert.Equal(t, "Nantucket", addr["city"].(string), "Unexpected city: %v", addr["city"])
	assert.Equal(t, "MA", addr["state"].(string), "Unexpected state: %v", addr["state"])

	_, ok = addr["country"]
	assert.False(t, ok, "The country is not left out.")

	det, ok := dst["details"].(map[string]any)
	require.Truef(t, ok, "Details is the wrong type: %v", dst["details"])

	_, ok = det["friends"]
	assert.True(t, ok, "Could not find your friends. Maybe you don't have any. :-(")
	assert.Equal(t, "pequod", dst["boat"].(string), "Expected boat string, got %v", dst["boat"])

	_, ok = dst["hole"]
	assert.False(t, ok, "The hole still exists.")

	dst2 := map[string]any{
		"name": "Ishmael",
		"address": map[string]any{
			"street":  "123 Spouter Inn Ct.",
			"city":    "Nantucket",
			"country": "US",
		},
		"details": map[string]any{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
		"hole": "black",
	}

	// What we expect is that anything in dst should have all values set,
	// this happens when the --reuse-values flag is set but the chart has no modifications yet
	CoalesceTables(dst2, nil)

	assert.Equal(t, "Ishmael", dst2["name"], "Unexpected name: %s", dst2["name"])

	addr2, ok := dst2["address"].(map[string]any)
	require.True(t, ok, "Address went away.")
	assert.Equal(t, "123 Spouter Inn Ct.", addr2["street"].(string), "Unexpected address: %v", addr2["street"])
	assert.Equal(t, "Nantucket", addr2["city"].(string), "Unexpected city: %v", addr2["city"])
	assert.Equal(t, "US", addr2["country"].(string), "Unexpected Country: %v", addr2["country"])

	det2, ok := dst2["details"].(map[string]any)
	require.Truef(t, ok, "Details is the wrong type: %v", dst2["details"])

	_, ok = det2["friends"]
	assert.True(t, ok, "Could not find your friends. Maybe you don't have any. :-(")
	assert.Equal(t, "pequod", dst2["boat"].(string), "Expected boat string, got %v", dst2["boat"])
	assert.Equal(t, "black", dst2["hole"].(string), "Expected hole string, got %v", dst2["boat"])
}

func TestMergeTables(t *testing.T) {
	dst := map[string]any{
		"name": "Ishmael",
		"address": map[string]any{
			"street":  "123 Spouter Inn Ct.",
			"city":    "Nantucket",
			"country": nil,
		},
		"details": map[string]any{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
		"hole": nil,
	}
	src := map[string]any{
		"occupation": "whaler",
		"address": map[string]any{
			"state":   "MA",
			"street":  "234 Spouter Inn Ct.",
			"country": "US",
		},
		"details": "empty",
		"boat": map[string]any{
			"mast": true,
		},
		"hole": "black",
	}

	// What we expect is that anything in dst overrides anything in src, but that
	// otherwise the values are coalesced.
	MergeTables(dst, src)

	assert.Equal(t, "Ishmael", dst["name"], "Unexpected name: %s", dst["name"])
	assert.Equal(t, "whaler", dst["occupation"], "Unexpected occupation: %s", dst["occupation"])

	addr, ok := dst["address"].(map[string]any)
	require.True(t, ok, "Address went away.")
	assert.Equal(t, "123 Spouter Inn Ct.", addr["street"].(string), "Unexpected address: %v", addr["street"])
	assert.Equal(t, "Nantucket", addr["city"].(string), "Unexpected city: %v", addr["city"])
	assert.Equal(t, "MA", addr["state"].(string), "Unexpected state: %v", addr["state"])

	// This is one test that is different from CoalesceTables. Because country
	// is a nil value and it's not removed it's still present.
	_, ok = addr["country"]
	assert.True(t, ok, "The country is left out.")

	det, ok := dst["details"].(map[string]any)
	require.Truef(t, ok, "Details is the wrong type: %v", dst["details"])

	_, ok = det["friends"]
	assert.True(t, ok, "Could not find your friends. Maybe you don't have any. :-(")
	assert.Equal(t, "pequod", dst["boat"].(string), "Expected boat string, got %v", dst["boat"])

	// This is one test that is different from CoalesceTables. Because hole
	// is a nil value and it's not removed it's still present.
	assert.Contains(t, dst, "hole", "The hole no longer exists.")

	dst2 := map[string]any{
		"name": "Ishmael",
		"address": map[string]any{
			"street":  "123 Spouter Inn Ct.",
			"city":    "Nantucket",
			"country": "US",
		},
		"details": map[string]any{
			"friends": []string{"Tashtego"},
		},
		"boat":   "pequod",
		"hole":   "black",
		"nilval": nil,
	}

	// What we expect is that anything in dst should have all values set,
	// this happens when the --reuse-values flag is set but the chart has no modifications yet
	MergeTables(dst2, nil)

	assert.Equal(t, "Ishmael", dst2["name"], "Unexpected name: %s", dst2["name"])

	addr2, ok := dst2["address"].(map[string]any)
	require.True(t, ok, "Address went away.")
	assert.Equal(t, "123 Spouter Inn Ct.", addr2["street"].(string), "Unexpected address: %v", addr2["street"])
	assert.Equal(t, "Nantucket", addr2["city"].(string), "Unexpected city: %v", addr2["city"])
	assert.Equal(t, "US", addr2["country"].(string), "Unexpected Country: %v", addr2["country"])

	det2, ok := dst2["details"].(map[string]any)
	require.Truef(t, ok, "Details is the wrong type: %v", dst2["details"])

	assert.Contains(t, det2, "friends", "Could not find your friends. Maybe you don't have any. :-(")
	assert.Equal(t, "pequod", dst2["boat"].(string), "Expected boat string, got %v", dst2["boat"])
	assert.Equal(t, "black", dst2["hole"].(string), "Expected hole string, got %v", dst2["hole"])
	assert.Nil(t, dst2["nilval"], "Expected nilvalue to have nil value but it does not")
}

func TestCoalesceValuesWarnings(t *testing.T) {
	c := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "level1"},
		Values: map[string]any{
			"name": "moby",
		},
	},
		withDeps(&chart.Chart{
			Metadata: &chart.Metadata{Name: "level2"},
			Values: map[string]any{
				"name": "pequod",
			},
		},
			&chart.Chart{
				Metadata: &chart.Metadata{Name: "level3"},
				Values: map[string]any{
					"name": "ahab",
					"boat": true,
					"spear": map[string]any{
						"tip": true,
						"sail": map[string]any{
							"cotton": true,
						},
					},
				},
			},
		),
	)

	vals := map[string]any{
		"level2": map[string]any{
			"level3": map[string]any{
				"boat": map[string]any{"mast": true},
				"spear": map[string]any{
					"tip": map[string]any{
						"sharp": true,
					},
					"sail": true,
				},
			},
		},
	}

	warnings := make([]string, 0)
	printf := func(format string, v ...any) {
		t.Logf(format, v...)
		warnings = append(warnings, fmt.Sprintf(format, v...))
	}

	_, err := coalesce(printf, c, vals, "", false)
	require.NoError(t, err)

	t.Logf("vals: %v", vals)
	assert.Contains(t, warnings, "warning: skipped value for level1.level2.level3.boat: Not a table.")
	assert.Contains(t, warnings, "warning: destination for level1.level2.level3.spear.tip is a table. Ignoring non-table value (true)")
	assert.Contains(t, warnings, "warning: cannot overwrite table with non table for level1.level2.level3.spear.sail (map[cotton:true])")
}

func TestConcatPrefix(t *testing.T) {
	assert.Equal(t, "b", concatPrefix("", "b"))
	assert.Equal(t, "a.b", concatPrefix("a", "b"))
}

// TestCoalesceValuesEmptyMapWithNils tests the full CoalesceValues scenario
// from issue #31643 where chart has data: {} and user provides data: {foo: bar, baz: ~}
func TestCoalesceValuesEmptyMapWithNils(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
		Values: map[string]any{
			"data": map[string]any{}, // empty map in chart defaults
		},
	}

	vals := map[string]any{
		"data": map[string]any{
			"foo": "bar",
			"baz": nil, // explicit nil from user
		},
	}

	v, err := CoalesceValues(c, vals)
	req.NoError(err)

	data, ok := v["data"].(map[string]any)
	is.True(ok, "data is not a map")

	// "foo" should be preserved
	is.Equal("bar", data["foo"])

	// "baz" should be preserved with nil value since it wasn't in chart defaults
	is.Contains(data, "baz", "Expected data.baz key to be present but it was removed")
	is.Nil(data["baz"], "Expected data.baz key to be nil but it is not")
}

// TestCoalesceValuesSubchartDefaultNilsCleaned tests that nil values in subchart defaults
// are cleaned up during coalescing when the parent doesn't set those keys.
// Regression test for issue #31919.
func TestCoalesceValuesSubchartDefaultNilsCleaned(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	// Subchart has a default with nil values (e.g. keyMapping: {password: null})
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "child"},
		Values: map[string]any{
			"keyMapping": map[string]any{
				"password": nil,
			},
		},
	}

	parent := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "parent"},
		Values:   map[string]any{},
	}, subchart)

	// Parent user values don't mention keyMapping at all
	vals := map[string]any{}

	v, err := CoalesceValues(parent, vals)
	req.NoError(err)

	childVals, ok := v["child"].(map[string]any)
	is.True(ok, "child values should be a map")

	keyMapping, ok := childVals["keyMapping"].(map[string]any)
	is.True(ok, "keyMapping should be a map")

	// The nil "password" key from chart defaults should be cleaned up
	_, ok = keyMapping["password"]
	is.False(ok, "Expected keyMapping.password (nil from chart defaults) to be removed, but it is still present")
}

// TestCoalesceValuesUserNullErasesSubchartDefault tests that a user-supplied null
// value erases a subchart's default value during coalescing.
// Regression test for issue #31919.
func TestCoalesceValuesUserNullErasesSubchartDefault(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "child"},
		Values: map[string]any{
			"someKey": "default",
		},
	}

	parent := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "parent"},
		Values:   map[string]any{},
	}, subchart)

	// User explicitly nullifies the subchart key via parent values
	vals := map[string]any{
		"child": map[string]any{
			"someKey": nil,
		},
	}

	v, err := CoalesceValues(parent, vals)
	req.NoError(err)

	childVals, ok := v["child"].(map[string]any)
	is.True(ok, "child values should be a map")

	// someKey should be erased — user null overrides subchart default
	_, ok = childVals["someKey"]
	is.False(ok, "Expected someKey to be removed by user null override, but it is still present")
}

// TestCoalesceValuesSubchartNilDoesNotShadowGlobal tests that a nil value in
// subchart defaults doesn't shadow a global value accessible via pluck-like access.
// Regression test for issue #31971.
func TestCoalesceValuesSubchartNilDoesNotShadowGlobal(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "child"},
		Values: map[string]any{
			"ingress": map[string]any{
				"feature": nil, // nil in subchart defaults
			},
		},
	}

	parent := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "parent"},
		Values:   map[string]any{},
	}, subchart)

	// Parent sets the global value
	vals := map[string]any{
		"global": map[string]any{
			"ingress": map[string]any{
				"feature": true,
			},
		},
	}

	v, err := CoalesceValues(parent, vals)
	req.NoError(err)

	childVals, ok := v["child"].(map[string]any)
	is.True(ok, "child values should be a map")

	ingress, ok := childVals["ingress"].(map[string]any)
	is.True(ok, "ingress should be a map")

	// The nil "feature" from subchart defaults should be cleaned up,
	// so that pluck can fall through to the global value
	_, ok = ingress["feature"]
	is.False(ok, "Expected ingress.feature (nil from chart defaults) to be removed so global can be used via pluck, but it is still present")
}

// TestCoalesceValuesSubchartNilCleanedWhenUserPartiallyOverrides tests that nil
// values in subchart defaults are cleaned even when the user partially overrides
// the same map. Regression test for the coalesceTablesFullKey merge path.
func TestCoalesceValuesSubchartNilCleanedWhenUserPartiallyOverrides(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "child"},
		Values: map[string]any{
			"keyMapping": map[string]any{
				"password": nil,
				"format":   "bcrypt",
			},
		},
	}

	parent := withDeps(&chart.Chart{
		Metadata: &chart.Metadata{Name: "parent"},
		Values:   map[string]any{},
	}, subchart)

	// User overrides format but doesn't mention password
	vals := map[string]any{
		"child": map[string]any{
			"keyMapping": map[string]any{
				"format": "sha256",
			},
		},
	}

	v, err := CoalesceValues(parent, vals)
	req.NoError(err)

	childVals, ok := v["child"].(map[string]any)
	is.True(ok, "child values should be a map")

	keyMapping, ok := childVals["keyMapping"].(map[string]any)
	is.True(ok, "keyMapping should be a map")
	is.Equal("sha256", keyMapping["format"], "User override should be preserved")

	_, ok = keyMapping["password"]
	is.False(ok, "Expected keyMapping.password (nil from chart defaults) to be removed even when user partially overrides the map")
}
