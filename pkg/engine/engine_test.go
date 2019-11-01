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
	"sort"
	"strings"
	"sync"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
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

func TestParseErrors(t *testing.T) {
	vals := chartutil.Values{"Values": map[string]interface{}{}}

	tplsUndefinedFunction := map[string]renderable{
		"undefined_function": {tpl: `{{foo}}`, vals: vals},
	}
	_, err := new(Engine).render(tplsUndefinedFunction)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected := `parse error at (undefined_function:1): function "foo" not defined`
	if err.Error() != expected {
		t.Errorf("Expected '%s', got %q", expected, err.Error())
	}
}

func TestExecErrors(t *testing.T) {
	vals := chartutil.Values{"Values": map[string]interface{}{}}

	tplsMissingRequired := map[string]renderable{
		"missing_required": {tpl: `{{required "foo is required" .Values.foo}}`, vals: vals},
	}
	_, err := new(Engine).render(tplsMissingRequired)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected := `execution error at (missing_required:1:2): foo is required`
	if err.Error() != expected {
		t.Errorf("Expected '%s', got %q", expected, err.Error())
	}

	tplsMissingRequired = map[string]renderable{
		"missing_required_with_colons": {tpl: `{{required ":this: message: has many: colons:" .Values.foo}}`, vals: vals},
	}
	_, err = new(Engine).render(tplsMissingRequired)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected = `execution error at (missing_required_with_colons:1:2): :this: message: has many: colons:`
	if err.Error() != expected {
		t.Errorf("Expected '%s', got %q", expected, err.Error())
	}

	issue6044tpl := `{{ $someEmptyValue := "" }}
{{ $myvar := "abc" }}
{{- required (printf "%s: something is missing" $myvar) $someEmptyValue | repeat 0 }}`
	tplsMissingRequired = map[string]renderable{
		"issue6044": {tpl: issue6044tpl, vals: vals},
	}
	_, err = new(Engine).render(tplsMissingRequired)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected = `execution error at (issue6044:3:4): abc: something is missing`
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

	out, err := Render(ch, map[string]interface{}{
		"Values": map[string]interface{}{},
	})
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
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{},
		"Chart":  outer.Metadata,
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{},
		"Chart":  c.Metadata,
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{
			"who":   "us",
			"bases": 2,
		},
		"Chart": c.Metadata,
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{
			"who": "us",
		},
		"Chart": c.Metadata,
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": map[string]interface{}{
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
		"Values": map[string]interface{}{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": map[string]interface{}{
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

func TestUpdateRenderValues_values_templates(t *testing.T) {
	values := map[string]interface{}{}
	rv := map[string]interface{}{
		"Release": map[string]interface{}{
			"Name": "Test Name",
		},
		"Values": values,
	}
	c := loadChart(t, "testdata/values_templates")

	if err := new(Engine).updateRenderValues(c, rv); err != nil {
		t.Fatal(err)
	}
	if v, ok := values["releaseName"]; !ok {
		t.Errorf("field 'releaseName' missing")
	} else if vs, ok := v.(string); !ok || vs != "Test Name" {
		t.Errorf("wrong value on field 'releaseName': %v", v)
	}
	// Check root remplacements
	if v, ok := values["replaced"]; !ok {
		t.Errorf("field 'replaced' missing")
	} else if vs := v.(string); vs != "values/replaced2.yaml" {
		t.Errorf("wrong priority on field 'replaced', value from %s", vs)
	}
	if v, ok := values["currentReplaced1"]; !ok {
		t.Errorf("field 'currentReplaced1' missing")
	} else if vs := v.(string); vs != "values.yaml" {
		t.Errorf("wrong evaluation order on field 'currentReplaced1', value from %s", vs)
	}
	if v, ok := values["currentReplaced2"]; !ok {
		t.Errorf("field 'currentReplaced2' missing")
	} else if vs := v.(string); vs != "values.yaml" {
		t.Errorf("wrong evaluation order on field 'currentReplaced2', value from %s", vs)
	}
	// check root coalesce
	if vm, ok := values["coalesce"]; !ok {
		t.Errorf("field 'coalesce' missing")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["old"]; !ok {
			t.Errorf("field 'coalesce.old' missing")
		} else if vs := v.(string); vs != "values.yaml" {
			t.Errorf("wrong priority on field 'coalesce.old', value from %s", vs)
		}
		if v, ok := m["common"]; !ok {
			t.Errorf("field 'coalesce.common' missing")
		} else if vs := v.(string); vs != "values/coalesce.yaml" {
			t.Errorf("wrong priority on field 'coalesce.common', value from %s", vs)
		}
		if v, ok := m["new"]; !ok {
			t.Errorf("field 'coalesce.new' missing")
		} else if vs := v.(string); vs != "values/coalesce.yaml" {
			t.Errorf("wrong priority on field 'coalesce.new', value from %s", vs)
		}
	}
	// check root global
	if vm, ok := values["global"]; !ok {
		t.Errorf("field 'global' missing")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["parentValues"]; !ok || !v.(bool) {
			t.Errorf("field 'global.parentValues' missing")
		}
		if v, ok := m["parentTemplate"]; !ok || !v.(bool) {
			t.Errorf("field 'global.parentTemplate' missing")
		}
		if _, ok := m["subValues"]; ok {
			t.Errorf("field 'global.subValues' unexpected")
		}
		if _, ok := m["subTeamplate"]; ok {
			t.Errorf("field 'global.subTeamplate' unexpected")
		}
	}
	// check subchart
	if vm, ok := values["subchart"]; !ok {
		t.Errorf("field 'subchart' missing")
	} else {
		// check subchart evaluated
		m := vm.(map[string]interface{})
		if v, ok := m["evaluated"]; !ok || !v.(bool) {
			t.Errorf("chart 'subchart' not evaluated")
		}
		// check subchart replaced
		if v, ok := m["replaced1"]; !ok {
			t.Errorf("field 'subchart.replaced1' missing")
		} else if vs := v.(string); vs != "values.yaml" {
			t.Errorf("wrong priority on field 'subchart.replaced1', value from %s", vs)
		}
		if v, ok := m["replaced2"]; !ok {
			t.Errorf("field 'subchart.replaced2' missing")
		} else if vs := v.(string); vs != "subchart/values/replaced.yaml" {
			t.Errorf("wrong priority on field 'subchart.replaced2', value from %s", vs)
		}
		if v, ok := m["replaced3"]; !ok {
			t.Errorf("field 'subchart.replaced3' missing")
		} else if vs := v.(string); vs != "values/sub_replaced.yaml" {
			t.Errorf("wrong priority on field 'subchart.replaced3', value from %s", vs)
		}
		if v, ok := m["replaced4"]; !ok {
			t.Errorf("field 'subchart.replaced4' missing")
		} else if vs := v.(string); vs != "subchart/values/replaced.yaml" {
			t.Errorf("wrong priority on field 'subchart.replaced4', value from %s", vs)
		}
		if v, ok := m["currentReplaced2"]; !ok {
			t.Errorf("field 'subchart.currentReplaced2' missing")
		} else if vs := v.(string); vs != "values.yaml" {
			t.Errorf("wrong evaluation order on field 'subchart.currentReplaced2', value from %s", vs)
		}
		// check subchart coalesce
		if vm, ok := m["coalesce"]; !ok {
			t.Errorf("field 'subchart.coalesce' missing")
		} else {
			m := vm.(map[string]interface{})
			if v, ok := m["value1"]; !ok {
				t.Errorf("field 'subchart.coalesce.value1' missing")
			} else if vs := v.(string); vs != "values.yaml" {
				t.Errorf("wrong priority on field 'subchart.coalesce.value1', value from %s", vs)
			}
			if v, ok := m["value2"]; !ok {
				t.Errorf("field 'subchart.coalesce.value2' missing")
			} else if vs := v.(string); vs != "values.yaml" {
				t.Errorf("wrong priority on field 'subchart.coalesce.value2', value from %s", vs)
			}
			if v, ok := m["value3"]; !ok {
				t.Errorf("field 'subchart.coalesce.value3' missing")
			} else if vs := v.(string); vs != "subchart/values/coalesce.yaml" {
				t.Errorf("wrong priority on field 'subchart.coalesce.value3', value from %s", vs)
			}
			if v, ok := m["value4"]; !ok {
				t.Errorf("field 'subchart.coalesce.value4' missing")
			} else if vs := v.(string); vs != "subchart/values.yaml" {
				t.Errorf("wrong priority on field 'subchart.coalesce.value4', value from %s", vs)
			}
			if v, ok := m["value5"]; !ok {
				t.Errorf("field 'subchart.coalesce.value5' missing")
			} else if vs := v.(string); vs != "subchart/values/coalesce.yaml" {
				t.Errorf("wrong priority on field 'subchart.coalesce.value5', value from %s", vs)
			}
			if v, ok := m["value6"]; !ok {
				t.Errorf("field 'subchart.coalesce.value6' missing")
			} else if vs := v.(string); vs != "subchart/values/coalesce.yaml" {
				t.Errorf("wrong priority on field 'subchart.coalesce.value6', value from %s", vs)
			}
		}
		// check subchart global
		if vm, ok := m["global"]; !ok {
			t.Errorf("field 'subchart.global' missing")
		} else {
			m := vm.(map[string]interface{})
			if v, ok := m["parentValues"]; !ok || !v.(bool) {
				t.Errorf("field 'subchart.global.parentValues' missing")
			}
			if v, ok := m["parentTemplate"]; !ok || !v.(bool) {
				t.Errorf("field 'subchart.global.parentTemplate' missing")
			}
			if v, ok := m["subTeamplate"]; !ok || !v.(bool) {
				t.Errorf("field 'subchart.global.subTeamplate' missing")
			}
			if v, ok := m["parentValues"]; !ok || !v.(bool) {
				t.Errorf("field 'subchart.global.parentValues' missing")
			}
		}
		// check subchart globalEvaluated
		if vm, ok := m["globalEvaluated"]; !ok {
			t.Errorf("field 'subchart.globalEvaluated' missing")
		} else {
			m := vm.(map[string]interface{})
			if v, ok := m["parentValues"]; !ok {
				t.Errorf("field 'subchart.globalEvaluated.parentValues' missing")
			} else if vb, ok := v.(bool); !ok || !vb {
				t.Errorf("field 'subchart.globalEvaluated.parentValues' has wrong value: %v", vb)
			}
			if v, ok := m["parentTemplate"]; !ok {
				t.Errorf("field 'subchart.globalEvaluated.parentTemplate' missing")
			} else if vb, ok := v.(bool); !ok || !vb {
				t.Errorf("field 'subchart.globalEvaluated.parentTemplate' has wrong value: %v", vb)
			}
			if v, ok := m["subValues"]; !ok {
				t.Errorf("field 'subchart.globalEvaluated.subValues' missing")
			} else if vb, ok := v.(bool); !ok || !vb {
				t.Errorf("field 'subchart.globalEvaluated.subValues' has wrong value: %v", vb)
			}
			if v, ok := m["subTeamplate"]; !ok {
				t.Errorf("field 'subchart.globalEvaluated.subTeamplate' missing")
			} else if v != nil {
				t.Errorf("field 'subchart.globalEvaluated.subTeamplate' has wrong value: %v", v)
			}
		}
	}
}

func TestUpdateRenderValues_dependencies(t *testing.T) {
	values := map[string]interface{}{}
	rv := map[string]interface{}{
		"Release": map[string]interface{}{
			"Name": "Test Name",
		},
		"Values": values,
	}
	c := loadChart(t, "testdata/dependencies")

	if err := new(Engine).updateRenderValues(c, rv); err != nil {
		t.Fatal(err)
	}
	// check for conditions
	if vm, ok := values["condition_true"]; !ok {
		t.Errorf("chart 'condition_true' not evaluated")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["evaluated"]; !ok || !v.(bool) {
			t.Errorf("chart 'condition_true' not evaluated")
		}
	}
	if _, ok := values["condition_false"]; ok {
		t.Errorf("chart 'condition_false' evaluated")
	}
	if vm, ok := values["condition_null"]; !ok {
		t.Errorf("chart 'condition_null' not evaluated")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["evaluated"]; !ok || !v.(bool) {
			t.Errorf("chart 'condition_null' not evaluated")
		}
	}
	// check for tags
	if vm, ok := values["tags_true"]; !ok {
		t.Errorf("chart 'tags_true' not evaluated")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["evaluated"]; !ok || !v.(bool) {
			t.Errorf("chart 'tags_true' not evaluated")
		}
	}
	if _, ok := values["tags_false"]; ok {
		t.Errorf("chart 'tags_false' evaluated")
	}
	// check for sub tags
	if vm, ok := values["tags_sub"]; !ok {
		t.Errorf("chart 'tags_sub' not evaluated")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["evaluated"]; !ok || !v.(bool) {
			t.Errorf("chart 'tags_sub' not evaluated")
		}
		if vm, ok := m["tags_sub_true"]; !ok {
			t.Errorf("chart 'tags_sub/tags_sub_true' not evaluated")
		} else {
			m := vm.(map[string]interface{})
			if v, ok := m["evaluated"]; !ok || !v.(bool) {
				t.Errorf("chart 'tags_sub/tags_sub_true' not evaluated")
			}
		}
		if _, ok := m["tags_sub/tags_sub_false"]; ok {
			t.Errorf("chart 'tags_sub/tags_sub_false' evaluated")
		}
	}
	// check for import-values
	if vm, ok := values["import_values"]; !ok {
		t.Errorf("chart 'import_values' not evaluated")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["evaluated"]; !ok || !v.(bool) {
			t.Errorf("chart 'import_values' not evaluated")
		}
	}
	if vm, ok := values["importValues"]; !ok {
		t.Errorf("value 'importValues' not imported")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["imported"]; !ok || !v.(bool) {
			t.Errorf("value 'importValues.imported' not imported")
		}
	}
	if vm, ok := values["importTemplate"]; !ok {
		t.Errorf("value 'importTemplate' not imported")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["imported"]; !ok || !v.(bool) {
			t.Errorf("value 'importTemplate.imported' not imported")
		}
	}
	if vm, ok := values["subImport"]; !ok {
		t.Errorf("value 'subImport' not imported")
	} else {
		m := vm.(map[string]interface{})
		if v, ok := m["old"]; !ok {
			t.Errorf("value 'importTemplate.old' not imported")
		} else if vs, ok := v.(string); !ok || vs != "values.yaml" {
			t.Errorf("wrong 'importTemplate.old' imported: %v", v)
		}
		if v, ok := m["common"]; !ok {
			t.Errorf("value 'importTemplate.common' not imported")
		} else if vs, ok := v.(string); !ok || vs != "values/import.yaml" {
			t.Errorf("wrong 'importTemplate.common' imported: %v", v)
		}
		if v, ok := m["new"]; !ok {
			t.Errorf("value 'importTemplate.new' not imported")
		} else if vs, ok := v.(string); !ok || vs != "values/import.yaml" {
			t.Errorf("wrong 'importTemplate.new' imported: %v", v)
		}
	}

	names := extractChartNames(c)
	except := []string{
		"parentchart",
		"parentchart.condition_null",
		"parentchart.condition_true",
		"parentchart.import_values",
		"parentchart.tags_sub",
		"parentchart.tags_sub.tags_sub_true",
		"parentchart.tags_true",
	}
	if len(names) != len(except) {
		t.Errorf("dependencies values do not match got %v, expected %v", names, except)
	} else {
		for i := range names {
			if names[i] != except[i] {
				t.Errorf("dependencies values do not match got %v, expected %v", names, except)
			}
			break
		}
	}
}

// copied from chartutil/values_test.go:TestToRenderValues
// because ToRenderValues no longer coalesces chart values
func TestUpdateRenderValues_ToRenderValues(t *testing.T) {

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
			"city":  "Baghdad",
			"date":  "809 CE",
			"title": "caliph",
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

	o := chartutil.ReleaseOptions{
		Name:      "Seven Voyages",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}

	res, err := chartutil.ToRenderValues(c, overideValues, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = new(Engine).updateRenderValues(c, res); err != nil {
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
	if namespace := relmap["Namespace"]; namespace.(string) != "default" {
		t.Errorf("Expected namespace 'default', got %q", namespace)
	}
	if revision := relmap["Revision"]; revision.(int) != 1 {
		t.Errorf("Expected revision '1', got %d", revision)
	}
	if relmap["IsUpgrade"].(bool) {
		t.Error("Expected upgrade to be false.")
	}
	if !relmap["IsInstall"].(bool) {
		t.Errorf("Expected install to be true.")
	}
	if !res["Capabilities"].(*chartutil.Capabilities).APIVersions.Has("v1") {
		t.Error("Expected Capabilities to have v1 as an API")
	}
	if res["Capabilities"].(*chartutil.Capabilities).KubeVersion.Major != "1" {
		t.Error("Expected Capabilities to have a Kube version")
	}

	vals := res["Values"].(map[string]interface{})
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

// copied from chartutil/dependencies_test.go:TestDependencyEnabled
// because ProcessDependencyEnabled is no longer recursive
func TestUpdateRenderValues_TestDependencyEnabled(t *testing.T) {
	type M = map[string]interface{}
	tests := []struct {
		name string
		v    M
		e    []string // expected charts including duplicates in alphanumeric order
	}{{
		"tags with no effect",
		M{"tags": M{"nothinguseful": false}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb"},
	}, {
		"tags disabling a group",
		M{"tags": M{"front-end": false}},
		[]string{"parentchart"},
	}, {
		"tags disabling a group and enabling a different group",
		M{"tags": M{"front-end": false, "back-end": true}},
		[]string{"parentchart", "parentchart.subchart2", "parentchart.subchart2.subchartb", "parentchart.subchart2.subchartc"},
	}, {
		"tags disabling only children, children still enabled since tag front-end=true in values.yaml",
		M{"tags": M{"subcharta": false, "subchartb": false}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb"},
	}, {
		"tags disabling all parents/children with additional tag re-enabling a parent",
		M{"tags": M{"front-end": false, "subchart1": true, "back-end": false}},
		[]string{"parentchart", "parentchart.subchart1"},
	}, {
		"conditions enabling the parent charts, but back-end (b, c) is still disabled via values.yaml",
		M{"subchart1": M{"enabled": true}, "subchart2": M{"enabled": true}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb", "parentchart.subchart2"},
	}, {
		"conditions disabling the parent charts, effectively disabling children",
		M{"subchart1": M{"enabled": false}, "subchart2": M{"enabled": false}},
		[]string{"parentchart"},
	}, {
		"conditions a child using the second condition path of child's condition",
		M{"subchart1": M{"subcharta": M{"enabled": false}}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subchartb"},
	}, {
		"tags enabling a parent/child group with condition disabling one child",
		M{"subchart2": M{"subchartc": M{"enabled": false}}, "tags": M{"back-end": true}},
		[]string{"parentchart", "parentchart.subchart1", "parentchart.subchart1.subcharta", "parentchart.subchart1.subchartb", "parentchart.subchart2", "parentchart.subchart2.subchartb"},
	}, {
		"tags will not enable a child if parent is explicitly disabled with condition",
		M{"subchart1": M{"enabled": false}, "tags": M{"front-end": true}},
		[]string{"parentchart"},
	}, {
		"subcharts with alias also respect conditions",
		M{"subchart1": M{"enabled": false}, "subchart2alias": M{"enabled": true, "subchartb": M{"enabled": true}}},
		[]string{"parentchart", "parentchart.subchart2alias", "parentchart.subchart2alias.subchartb"},
	}}

	for _, tc := range tests {
		c := loadChart(t, "../chartutil/testdata/subpop")
		vals := map[string]interface{}{"Values": tc.v}
		t.Run(tc.name, func(t *testing.T) {
			if err := new(Engine).updateRenderValues(c, vals); err != nil {
				t.Fatalf("error processing enabled dependencies %v", err)
			}

			names := extractChartNames(c)
			if len(names) != len(tc.e) {
				t.Fatalf("slice lengths do not match got %v, expected %v", len(names), len(tc.e))
			}
			for i := range names {
				if names[i] != tc.e[i] {
					t.Fatalf("slice values do not match got %v, expected %v", names, tc.e)
				}
			}
		})
	}
}

// copied from chartutil/dependencies_test.go:loadChart
func loadChart(t *testing.T, path string) *chart.Chart {
	t.Helper()
	c, err := loader.Load(path)
	if err != nil {
		t.Fatalf("failed to load testdata: %s", err)
	}
	return c
}

// copied from chartutil/dependencies_test.go:extractChartNames
// extractCharts recursively searches chart dependencies returning all charts found
func extractChartNames(c *chart.Chart) []string {
	var out []string
	var fn func(c *chart.Chart)
	fn = func(c *chart.Chart) {
		out = append(out, c.ChartPath())
		for _, d := range c.Dependencies() {
			fn(d)
		}
	}
	fn(c)
	sort.Strings(out)
	return out
}
