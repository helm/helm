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

package engine

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chartutil"
)

func TestSortTemplates(t *testing.T) {
	tpls := map[string]renderable{
		"/mychart/templates/foo.tpl":                                 {},
		"/mychart/templates/charts/foo/charts/bar/templates/foo.tpl": {},
		"/mychart/templates/bar.tpl":                                 {},
		"/mychart/templates/charts/foo/templates/bar.tpl":            {},
		"/mychart/templates/_foo.tpl":                                {},
		"/mychart/templates/charts/foo/templates/foo.tpl":            {},
		"/mychart/templates/charts/bar/templates/foo.tpl":            {},
	}
	got := sortTemplates(tpls)
	if len(got) != len(tpls) {
		t.Fatal("Sorted results are missing templates")
	}

	expect := []string{
		"/mychart/templates/charts/foo/charts/bar/templates/foo.tpl",
		"/mychart/templates/charts/foo/templates/foo.tpl",
		"/mychart/templates/charts/foo/templates/bar.tpl",
		"/mychart/templates/charts/bar/templates/foo.tpl",
		"/mychart/templates/foo.tpl",
		"/mychart/templates/bar.tpl",
		"/mychart/templates/_foo.tpl",
	}
	for i, e := range expect {
		if got[i] != e {
			t.Fatalf("\n\tExp:\n%s\n\tGot:\n%s",
				strings.Join(expect, "\n"),
				strings.Join(got, "\n"),
			)
		}
	}
}

func TestFuncMap(t *testing.T) {
	fns := funcMap()
	forbidden := []string{"env", "expandenv"}
	for _, f := range forbidden {
		if _, ok := fns[f]; ok {
			t.Errorf("Forbidden function %s exists in FuncMap.", f)
		}
	}

	// Test for Engine-specific template functions.
	expect := []string{"include", "required", "tpl", "toYaml", "fromYaml", "toToml", "toJson", "fromJson"}
	for _, f := range expect {
		if _, ok := fns[f]; !ok {
			t.Errorf("Expected add-on function %q", f)
		}
	}
}

func TestRender(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "moby",
			Version: "1.2.3",
		},
		Templates: []*chart.File{
			{Name: "templates/test1", Data: []byte("{{.Values.outer | title }} {{.Values.inner | title}}")},
			{Name: "templates/test2", Data: []byte("{{.Values.global.callme | lower }}")},
			{Name: "templates/test3", Data: []byte("{{.noValue}}")},
			{Name: "templates/test4", Data: []byte("{{toJson .Values}}")},
		},
		Values: map[string]interface{}{"outer": "DEFAULT", "inner": "DEFAULT"},
	}

	vals := map[string]interface{}{
		"Values": map[string]interface{}{
			"outer": "spouter",
			"inner": "inn",
			"global": map[string]interface{}{
				"callme": "Ishmael",
			},
		},
	}

	v, err := chartutil.CoalesceValues(c, vals)
	if err != nil {
		t.Fatalf("Failed to coalesce values: %s", err)
	}
	out, err := Render(c, v)
	if err != nil {
		t.Errorf("Failed to render templates: %s", err)
	}

	expect := map[string]string{
		"moby/templates/test1": "Spouter Inn",
		"moby/templates/test2": "ishmael",
		"moby/templates/test3": "",
		"moby/templates/test4": `{"global":{"callme":"Ishmael"},"inner":"inn","outer":"spouter"}`,
	}

	for name, data := range expect {
		if out[name] != data {
			t.Errorf("Expected %q, got %q", data, out[name])
		}
	}
}

func TestRenderInternals(t *testing.T) {
	// Test the internals of the rendering tool.

	vals := chartutil.Values{"Name": "one", "Value": "two"}
	tpls := map[string]renderable{
		"one": {tpl: `Hello {{title .Name}}`, vals: vals},
		"two": {tpl: `Goodbye {{upper .Value}}`, vals: vals},
		// Test whether a template can reliably reference another template
		// without regard for ordering.
		"three": {tpl: `{{template "two" dict "Value" "three"}}`, vals: vals},
	}

	out, err := new(Engine).render(tpls)
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
	e := new(Engine)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			tt := fmt.Sprintf("expect-%d", i)
			tpls := map[string]renderable{
				"t": {
					tpl:  `{{.val}}`,
					vals: map[string]interface{}{"val": tt},
				},
			}
			out, err := e.render(tpls)
			if err != nil {
				t.Errorf("Failed to render %s: %s", tt, err)
			}
			if out["t"] != tt {
				t.Errorf("Expected %q, got %q", tt, out["t"])
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestRenderErrors(t *testing.T) {
	vals := chartutil.Values{"Values": map[string]interface{}{}}

	tplsMissingRequired := map[string]renderable{
		"missing_required": {tpl: `{{required "foo is required" .Values.foo}}`, vals: vals},
	}
	_, err := new(Engine).render(tplsMissingRequired)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected := `render error at (missing_required:1:2): foo is required`
	if err.Error() != expected {
		t.Errorf("Expected '%s', got %q", expected, err.Error())
	}

	tplsUndefinedFunction := map[string]renderable{
		"undefined_function": {tpl: `{{foo}}`, vals: vals},
	}
	_, err = new(Engine).render(tplsUndefinedFunction)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected = `render error at (undefined_function:1): function "foo" not defined`
	if err.Error() != expected {
		t.Errorf("Expected '%s', got %q", expected, err.Error())
	}
}
func TestAllTemplates(t *testing.T) {
	ch1 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "ch1"},
		Templates: []*chart.File{
			{Name: "templates/foo", Data: []byte("foo")},
			{Name: "templates/bar", Data: []byte("bar")},
		},
	}
	dep1 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "laboratory mice"},
		Templates: []*chart.File{
			{Name: "templates/pinky", Data: []byte("pinky")},
			{Name: "templates/brain", Data: []byte("brain")},
		},
	}
	ch1.AddDependency(dep1)

	dep2 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "same thing we do every night"},
		Templates: []*chart.File{
			{Name: "templates/innermost", Data: []byte("innermost")},
		},
	}
	dep1.AddDependency(dep2)

	tpls := allTemplates(ch1, chartutil.Values{})
	if len(tpls) != 5 {
		t.Errorf("Expected 5 charts, got %d", len(tpls))
	}
}

func TestRenderDependency(t *testing.T) {
	deptpl := `{{define "myblock"}}World{{end}}`
	toptpl := `Hello {{template "myblock"}}`
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "outerchart"},
		Templates: []*chart.File{
			{Name: "templates/outer", Data: []byte(toptpl)},
		},
	}
	ch.AddDependency(&chart.Chart{
		Metadata: &chart.Metadata{Name: "innerchart"},
		Templates: []*chart.File{
			{Name: "templates/inner", Data: []byte(deptpl)},
		},
	})

	out, err := Render(ch, map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to render chart: %s", err)
	}

	if len(out) != 2 {
		t.Errorf("Expected 2, got %d", len(out))
	}

	expect := "Hello World"
	if out["outerchart/templates/outer"] != expect {
		t.Errorf("Expected %q, got %q", expect, out["outer"])
	}

}

func TestRenderNestedValues(t *testing.T) {
	innerpath := "templates/inner.tpl"
	outerpath := "templates/outer.tpl"
	// Ensure namespacing rules are working.
	deepestpath := "templates/inner.tpl"
	checkrelease := "templates/release.tpl"

	deepest := &chart.Chart{
		Metadata: &chart.Metadata{Name: "deepest"},
		Templates: []*chart.File{
			{Name: deepestpath, Data: []byte(`And this same {{.Values.what}} that smiles {{.Values.global.when}}`)},
			{Name: checkrelease, Data: []byte(`Tomorrow will be {{default "happy" .Release.Name }}`)},
		},
		Values: map[string]interface{}{"what": "milkshake"},
	}

	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "herrick"},
		Templates: []*chart.File{
			{Name: innerpath, Data: []byte(`Old {{.Values.who}} is still a-flyin'`)},
		},
		Values: map[string]interface{}{"who": "Robert"},
	}
	inner.AddDependency(deepest)

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "top"},
		Templates: []*chart.File{
			{Name: outerpath, Data: []byte(`Gather ye {{.Values.what}} while ye may`)},
		},
		Values: map[string]interface{}{
			"what": "stinkweed",
			"who":  "me",
			"herrick": map[string]interface{}{
				"who": "time",
			},
		},
	}
	outer.AddDependency(inner)

	injValues := map[string]interface{}{
		"what": "rosebuds",
		"herrick": map[string]interface{}{
			"deepest": map[string]interface{}{
				"what": "flower",
			},
		},
		"global": map[string]interface{}{
			"when": "to-day",
		},
	}

	tmp, err := chartutil.CoalesceValues(outer, injValues)
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

	out, err := Render(outer, inject)
	if err != nil {
		t.Fatalf("failed to render templates: %s", err)
	}

	fullouterpath := "top/" + outerpath
	if out[fullouterpath] != "Gather ye rosebuds while ye may" {
		t.Errorf("Unexpected outer: %q", out[fullouterpath])
	}

	fullinnerpath := "top/charts/herrick/" + innerpath
	if out[fullinnerpath] != "Old time is still a-flyin'" {
		t.Errorf("Unexpected inner: %q", out[fullinnerpath])
	}

	fulldeepestpath := "top/charts/herrick/charts/deepest/" + deepestpath
	if out[fulldeepestpath] != "And this same flower that smiles to-day" {
		t.Errorf("Unexpected deepest: %q", out[fulldeepestpath])
	}

	fullcheckrelease := "top/charts/herrick/charts/deepest/" + checkrelease
	if out[fullcheckrelease] != "Tomorrow will be dyin" {
		t.Errorf("Unexpected release: %q", out[fullcheckrelease])
	}
}

func TestRenderBuiltinValues(t *testing.T) {
	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "Latium"},
		Templates: []*chart.File{
			{Name: "templates/Lavinia", Data: []byte(`{{.Template.Name}}{{.Chart.Name}}{{.Release.Name}}`)},
			{Name: "templates/From", Data: []byte(`{{.Files.author | printf "%s"}} {{.Files.Get "book/title.txt"}}`)},
		},
		Files: []*chart.File{
			{Name: "author", Data: []byte("Virgil")},
			{Name: "book/title.txt", Data: []byte("Aeneid")},
		},
	}

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "Troy"},
		Templates: []*chart.File{
			{Name: "templates/Aeneas", Data: []byte(`{{.Template.Name}}{{.Chart.Name}}{{.Release.Name}}`)},
		},
	}
	outer.AddDependency(inner)

	inject := chartutil.Values{
		"Values": "",
		"Chart":  outer.Metadata,
		"Release": chartutil.Values{
			"Name": "Aeneid",
		},
	}

	t.Logf("Calculated values: %v", outer)

	out, err := Render(outer, inject)
	if err != nil {
		t.Fatalf("failed to render templates: %s", err)
	}

	expects := map[string]string{
		"Troy/charts/Latium/templates/Lavinia": "Troy/charts/Latium/templates/LaviniaLatiumAeneid",
		"Troy/templates/Aeneas":                "Troy/templates/AeneasTroyAeneid",
		"Troy/charts/Latium/templates/From":    "Virgil Aeneid",
	}
	for file, expect := range expects {
		if out[file] != expect {
			t.Errorf("Expected %q, got %q", expect, out[file])
		}
	}

}

func TestAlterFuncMap_include(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "conrad"},
		Templates: []*chart.File{
			{Name: "templates/quote", Data: []byte(`{{include "conrad/templates/_partial" . | indent 2}} dead.`)},
			{Name: "templates/_partial", Data: []byte(`{{.Release.Name}} - he`)},
		},
	}

	v := chartutil.Values{
		"Values": "",
		"Chart":  c.Metadata,
		"Release": chartutil.Values{
			"Name": "Mistah Kurtz",
		},
	}

	out, err := Render(c, v)
	if err != nil {
		t.Fatal(err)
	}

	expect := "  Mistah Kurtz - he dead."
	if got := out["conrad/templates/quote"]; got != expect {
		t.Errorf("Expected %q, got %q (%v)", expect, got, out)
	}
}

func TestAlterFuncMap_require(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "conan"},
		Templates: []*chart.File{
			{Name: "templates/quote", Data: []byte(`All your base are belong to {{ required "A valid 'who' is required" .Values.who }}`)},
			{Name: "templates/bases", Data: []byte(`All {{ required "A valid 'bases' is required" .Values.bases }} of them!`)},
		},
	}

	v := chartutil.Values{
		"Values": chartutil.Values{
			"who":   "us",
			"bases": 2,
		},
		"Chart": c.Metadata,
		"Release": chartutil.Values{
			"Name": "That 90s meme",
		},
	}

	out, err := Render(c, v)
	if err != nil {
		t.Fatal(err)
	}

	expectStr := "All your base are belong to us"
	if gotStr := out["conan/templates/quote"]; gotStr != expectStr {
		t.Errorf("Expected %q, got %q (%v)", expectStr, gotStr, out)
	}
	expectNum := "All 2 of them!"
	if gotNum := out["conan/templates/bases"]; gotNum != expectNum {
		t.Errorf("Expected %q, got %q (%v)", expectNum, gotNum, out)
	}

	// test required without passing in needed values with lint mode on
	// verifies lint replaces required with an empty string (should not fail)
	lintValues := chartutil.Values{
		"Values": chartutil.Values{
			"who": "us",
		},
		"Chart": c.Metadata,
		"Release": chartutil.Values{
			"Name": "That 90s meme",
		},
	}
	var e Engine
	e.LintMode = true
	out, err = e.Render(c, lintValues)
	if err != nil {
		t.Fatal(err)
	}

	expectStr = "All your base are belong to us"
	if gotStr := out["conan/templates/quote"]; gotStr != expectStr {
		t.Errorf("Expected %q, got %q (%v)", expectStr, gotStr, out)
	}
	expectNum = "All  of them!"
	if gotNum := out["conan/templates/bases"]; gotNum != expectNum {
		t.Errorf("Expected %q, got %q (%v)", expectNum, gotNum, out)
	}
}

func TestAlterFuncMap_tpl(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplFunction"},
		Templates: []*chart.File{
			{Name: "templates/base", Data: []byte(`Evaluate tpl {{tpl "Value: {{ .Values.value}}" .}}`)},
		},
	}

	v := chartutil.Values{
		"Values": chartutil.Values{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": chartutil.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	if err != nil {
		t.Fatal(err)
	}

	expect := "Evaluate tpl Value: myvalue"
	if got := out["TplFunction/templates/base"]; got != expect {
		t.Errorf("Expected %q, got %q (%v)", expect, got, out)
	}
}

func TestAlterFuncMap_tplfunc(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplFunction"},
		Templates: []*chart.File{
			{Name: "templates/base", Data: []byte(`Evaluate tpl {{tpl "Value: {{ .Values.value | quote}}" .}}`)},
		},
	}

	v := chartutil.Values{
		"Values": chartutil.Values{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": chartutil.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	if err != nil {
		t.Fatal(err)
	}

	expect := "Evaluate tpl Value: \"myvalue\""
	if got := out["TplFunction/templates/base"]; got != expect {
		t.Errorf("Expected %q, got %q (%v)", expect, got, out)
	}
}

func TestAlterFuncMap_tplinclude(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplFunction"},
		Templates: []*chart.File{
			{Name: "templates/base", Data: []byte(`{{ tpl "{{include ` + "`" + `TplFunction/templates/_partial` + "`" + ` .  | quote }}" .}}`)},
			{Name: "templates/_partial", Data: []byte(`{{.Template.Name}}`)},
		},
	}
	v := chartutil.Values{
		"Values": chartutil.Values{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": chartutil.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	if err != nil {
		t.Fatal(err)
	}

	expect := "\"TplFunction/templates/base\""
	if got := out["TplFunction/templates/base"]; got != expect {
		t.Errorf("Expected %q, got %q (%v)", expect, got, out)
	}

}
