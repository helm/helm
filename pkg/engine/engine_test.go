package engine

import (
	"fmt"
	"sync"
	"testing"

	chartutil "github.com/deis/tiller/pkg/chart"
	"github.com/deis/tiller/pkg/proto/hapi/chart"
)

func TestEngine(t *testing.T) {
	e := New()

	// Forbidden because they allow access to the host OS.
	forbidden := []string{"env", "expandenv"}
	for _, f := range forbidden {
		if _, ok := e.FuncMap[f]; ok {
			t.Errorf("Forbidden function %s exists in FuncMap.", f)
		}
	}
}

func TestRender(t *testing.T) {
	t.Skip()
}

func TestRenderInternals(t *testing.T) {
	// Test the internals of the rendering tool.
	e := New()

	vals := chartutil.Values{"Name": "one", "Value": "two"}
	tpls := map[string]renderable{
		"one": {tpl: `Hello {{title .Name}}`, vals: vals},
		"two": {tpl: `Goodbye {{upper .Value}}`, vals: vals},
		// Test whether a template can reliably reference another template
		// without regard for ordering.
		"three": {tpl: `{{template "two" dict "Value" "three"}}`, vals: vals},
	}

	out, err := e.render(tpls)
	if err != nil {
		t.Fatalf("Failed template rendering: %s", err)
	}

	if len(out) != 3 {
		t.Fatalf("Expected 3 templates, got %d", len(out))
	}

	if out["one"] != "Hello One" {
		t.Errorf("Expected 'Hello One', got %q", out["one"])
	}

	if out["two"] != "Goodbye TWO" {
		t.Errorf("Expected 'Goodbye TWO'. got %q", out["two"])
	}

	if out["three"] != "Goodbye THREE" {
		t.Errorf("Expected 'Goodbye THREE'. got %q", out["two"])
	}
}

func TestParallelRenderInternals(t *testing.T) {
	// Make sure that we can use one Engine to run parallel template renders.
	e := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			fname := "my/file/name"
			tt := fmt.Sprintf("expect-%d", i)
			v := chartutil.Values{"val": tt}
			tpls := map[string]renderable{fname: {tpl: `{{.val}}`, vals: v}}
			out, err := e.render(tpls)
			if err != nil {
				t.Errorf("Failed to render %s: %s", tt, err)
			}
			if out[fname] != tt {
				t.Errorf("Expected %q, got %q", tt, out[fname])
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestAllTemplates(t *testing.T) {
	ch1 := &chart.Chart{
		Templates: []*chart.Template{
			{Name: "foo", Data: []byte("foo")},
			{Name: "bar", Data: []byte("bar")},
		},
		Dependencies: []*chart.Chart{
			{
				Templates: []*chart.Template{
					{Name: "pinky", Data: []byte("pinky")},
					{Name: "brain", Data: []byte("brain")},
				},
				Dependencies: []*chart.Chart{
					{Templates: []*chart.Template{
						{Name: "innermost", Data: []byte("innermost")},
					}},
				},
			},
		},
	}

	var v chartutil.Values
	tpls := allTemplates(ch1, v)
	if len(tpls) != 5 {
		t.Errorf("Expected 5 charts, got %d", len(tpls))
	}
}

func TestRenderDependency(t *testing.T) {
	e := New()
	deptpl := `{{define "myblock"}}World{{end}}`
	toptpl := `Hello {{template "myblock"}}`
	ch := &chart.Chart{
		Templates: []*chart.Template{
			{Name: "outer", Data: []byte(toptpl)},
		},
		Dependencies: []*chart.Chart{
			{
				Templates: []*chart.Template{
					{Name: "inner", Data: []byte(deptpl)},
				},
			},
		},
	}

	out, err := e.Render(ch, nil)

	if err != nil {
		t.Fatalf("failed to render chart: %s", err)
	}

	if len(out) != 2 {
		t.Errorf("Expected 2, got %d", len(out))
	}

	expect := "Hello World"
	if out["outer"] != expect {
		t.Errorf("Expected %q, got %q", expect, out["outer"])
	}

}

func TestRenderNestedValues(t *testing.T) {
	e := New()

	innerpath := "charts/inner/templates/inner.tpl"
	outerpath := "templates/outer.tpl"
	deepestpath := "charts/inner/charts/deepest/templates/deepest.tpl"

	deepest := &chart.Chart{
		Metadata: &chart.Metadata{Name: "deepest"},
		Templates: []*chart.Template{
			{Name: deepestpath, Data: []byte(`And this same {{.what}} that smiles to-day`)},
		},
		Values: &chart.Config{Raw: `what = "milkshake"`},
	}

	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "herrick"},
		Templates: []*chart.Template{
			{Name: innerpath, Data: []byte(`Old {{.who}} is still a-flyin'`)},
		},
		Values:       &chart.Config{Raw: `who = "Robert"`},
		Dependencies: []*chart.Chart{deepest},
	}

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "top"},
		Templates: []*chart.Template{
			{Name: outerpath, Data: []byte(`Gather ye {{.what}} while ye may`)},
		},
		Values: &chart.Config{
			Raw: `what = "stinkweed"
	[herrick]
	who = "time"
	`},
		Dependencies: []*chart.Chart{inner},
	}

	inject := chart.Config{
		Raw: `
		what = "rosebuds"
		[herrick.deepest]
		what = "flower"`,
	}

	out, err := e.Render(outer, &inject)
	if err != nil {
		t.Fatalf("failed to render templates: %s", err)
	}

	if out[outerpath] != "Gather ye rosebuds while ye may" {
		t.Errorf("Unexpected outer: %q", out[outerpath])
	}

	if out[innerpath] != "Old time is still a-flyin'" {
		t.Errorf("Unexpected inner: %q", out[innerpath])
	}

	if out[deepestpath] != "And this same flower that smiles to-day" {
		t.Errorf("Unexpected deepest: %q", out[deepestpath])
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
	coalesceTables(dst, src)

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

	if addr["street"].(string) != "234 Spouter Inn Ct." {
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
