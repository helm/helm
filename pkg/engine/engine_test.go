package engine

import (
	"fmt"
	"sync"
	"testing"

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

	tpls := map[string]string{
		"one": `Hello {{title .Name}}`,
		"two": `Goodbye {{upper .Value}}`,
		// Test whether a template can reliably reference another template
		// without regard for ordering.
		"three": `{{template "two" dict "Value" "three"}}`,
	}
	vals := map[string]string{"Name": "one", "Value": "two"}

	out, err := e.render(tpls, vals)
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
			tpls := map[string]string{fname: `{{.val}}`}
			v := map[string]string{"val": tt}
			out, err := e.render(tpls, v)
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

	tpls := allTemplates(ch1)
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
