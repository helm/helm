/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package engine

import (
	"fmt"
	"sync"
	"testing"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
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
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "moby",
			Version: "1.2.3",
		},
		Templates: []*chart.Template{
			{Name: "test1", Data: []byte("{{.outer | title }} {{.inner | title}}")},
			{Name: "test2", Data: []byte("{{.global.callme | lower }}")},
			{Name: "test3", Data: []byte("{{.noValue}}")},
		},
		Values: &chart.Config{
			Raw: "outer: DEFAULT\ninner: DEFAULT",
		},
	}

	vals := &chart.Config{
		Raw: `
outer: spouter
inner: inn
global:
  callme: Ishmael
`}

	e := New()
	v, err := chartutil.CoalesceValues(c, vals)
	if err != nil {
		t.Fatalf("Failed to coalesce values: %s", err)
	}
	out, err := e.Render(c, v)
	if err != nil {
		t.Errorf("Failed to render templates: %s", err)
	}

	expect := "Spouter Inn"
	if out["moby/test1"] != expect {
		t.Errorf("Expected %q, got %q", expect, out["test1"])
	}

	expect = "ishmael"
	if out["moby/test2"] != expect {
		t.Errorf("Expected %q, got %q", expect, out["test2"])
	}
	expect = ""
	if out["moby/test3"] != expect {
		t.Errorf("Expected %q, got %q", expect, out["test3"])
	}

	if _, err := e.Render(c, v); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
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
		Metadata: &chart.Metadata{Name: "ch1"},
		Templates: []*chart.Template{
			{Name: "foo", Data: []byte("foo")},
			{Name: "bar", Data: []byte("bar")},
		},
		Dependencies: []*chart.Chart{
			{
				Metadata: &chart.Metadata{Name: "laboratory mice"},
				Templates: []*chart.Template{
					{Name: "pinky", Data: []byte("pinky")},
					{Name: "brain", Data: []byte("brain")},
				},
				Dependencies: []*chart.Chart{{
					Metadata: &chart.Metadata{Name: "same thing we do every night"},
					Templates: []*chart.Template{
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
		Metadata: &chart.Metadata{Name: "outerchart"},
		Templates: []*chart.Template{
			{Name: "outer", Data: []byte(toptpl)},
		},
		Dependencies: []*chart.Chart{
			{
				Metadata: &chart.Metadata{Name: "innerchart"},
				Templates: []*chart.Template{
					{Name: "inner", Data: []byte(deptpl)},
				},
			},
		},
	}

	out, err := e.Render(ch, map[string]interface{}{})

	if err != nil {
		t.Fatalf("failed to render chart: %s", err)
	}

	if len(out) != 2 {
		t.Errorf("Expected 2, got %d", len(out))
	}

	expect := "Hello World"
	if out["outerchart/outer"] != expect {
		t.Errorf("Expected %q, got %q", expect, out["outer"])
	}

}

func TestRenderNestedValues(t *testing.T) {
	e := New()

	innerpath := "templates/inner.tpl"
	outerpath := "templates/outer.tpl"
	// Ensure namespacing rules are working.
	deepestpath := "templates/inner.tpl"
	checkrelease := "templates/release.tpl"

	deepest := &chart.Chart{
		Metadata: &chart.Metadata{Name: "deepest"},
		Templates: []*chart.Template{
			{Name: deepestpath, Data: []byte(`And this same {{.Values.what}} that smiles {{.Values.global.when}}`)},
			{Name: checkrelease, Data: []byte(`Tomorrow will be {{default "happy" .Release.Name }}`)},
		},
		Values: &chart.Config{Raw: `what: "milkshake"`},
	}

	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "herrick"},
		Templates: []*chart.Template{
			{Name: innerpath, Data: []byte(`Old {{.Values.who}} is still a-flyin'`)},
		},
		Values:       &chart.Config{Raw: `who: "Robert"`},
		Dependencies: []*chart.Chart{deepest},
	}

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "top"},
		Templates: []*chart.Template{
			{Name: outerpath, Data: []byte(`Gather ye {{.Values.what}} while ye may`)},
		},
		Values: &chart.Config{
			Raw: `
what: stinkweed
who: me
herrick:
    who: time`,
		},
		Dependencies: []*chart.Chart{inner},
	}

	injValues := chart.Config{
		Raw: `
what: rosebuds
herrick:
  deepest:
    what: flower
global:
  when: to-day`,
	}

	tmp, err := chartutil.CoalesceValues(outer, &injValues)
	if err != nil {
		t.Fatalf("Failed to coalesce values: %s", err)
	}

	inject := chartutil.Values{
		"Values": tmp,
		"Chart":  outer.Metadata,
		"Release": chartutil.Values{
			"Name": "dyin",
		},
	}

	t.Logf("Calculated values: %v", inject)

	out, err := e.Render(outer, inject)
	if err != nil {
		t.Fatalf("failed to render templates: %s", err)
	}

	if out["top/"+outerpath] != "Gather ye rosebuds while ye may" {
		t.Errorf("Unexpected outer: %q", out[outerpath])
	}

	if out["top/charts/herrick/"+innerpath] != "Old time is still a-flyin'" {
		t.Errorf("Unexpected inner: %q", out[innerpath])
	}

	if out["top/charts/herrick/charts/deepest/"+deepestpath] != "And this same flower that smiles to-day" {
		t.Errorf("Unexpected deepest: %q", out[deepestpath])
	}

	if out["top/charts/herrick/charts/deepest/"+checkrelease] != "Tomorrow will be dyin" {
		t.Errorf("Unexpected release: %q", out[checkrelease])
	}
}

func TestRenderBuiltinValues(t *testing.T) {
	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "Latium"},
		Templates: []*chart.Template{
			{Name: "Lavinia", Data: []byte(`{{.Template.Name}}{{.Chart.Name}}{{.Release.Name}}`)},
		},
		Values:       &chart.Config{Raw: ``},
		Dependencies: []*chart.Chart{},
	}

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "Troy"},
		Templates: []*chart.Template{
			{Name: "Aeneas", Data: []byte(`{{.Template.Name}}{{.Chart.Name}}{{.Release.Name}}`)},
		},
		Values:       &chart.Config{Raw: ``},
		Dependencies: []*chart.Chart{inner},
	}

	inject := chartutil.Values{
		"Values": &chart.Config{Raw: ""},
		"Chart":  outer.Metadata,
		"Release": chartutil.Values{
			"Name": "Aeneid",
		},
	}

	t.Logf("Calculated values: %v", outer)

	out, err := New().Render(outer, inject)
	if err != nil {
		t.Fatalf("failed to render templates: %s", err)
	}

	expects := map[string]string{
		"Troy/charts/Latium/Lavinia": "Troy/charts/Latium/LaviniaLatiumAeneid",
		"Troy/Aeneas":                "Troy/AeneasTroyAeneid",
	}
	for file, expect := range expects {
		if out[file] != expect {
			t.Errorf("Expected %q, got %q", expect, out[file])
		}
	}

}
