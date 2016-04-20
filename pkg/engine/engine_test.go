package engine

import (
	"fmt"
	"sync"
	"testing"
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
