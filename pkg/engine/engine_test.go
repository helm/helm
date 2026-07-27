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
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
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
	require.Len(t, got, len(tpls), "Sorted results are missing templates")

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
		require.Equal(t, e, got[i], "\n\tExp:\n%s\n\tGot:\n%s", strings.Join(expect, "\n"), strings.Join(got, "\n"))
	}
}

func TestFuncMap(t *testing.T) {
	fns := funcMap()
	forbidden := []string{"env", "expandenv"}
	for _, f := range forbidden {
		_, ok := fns[f]
		assert.Falsef(t, ok, "Forbidden function %s exists in FuncMap.", f)
	}

	// Test for Engine-specific template functions.
	expect := []string{"include", "required", "tpl", "toYaml", "fromYaml", "toToml", "fromToml", "toJson", "fromJson", "lookup"}
	for _, f := range expect {
		assert.Containsf(t, fns, f, "Expected add-on function %q", f)
	}
}

func TestRender(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "moby",
			Version: "1.2.3",
		},
		Templates: []*common.File{
			{Name: "templates/test1", ModTime: modTime, Data: []byte("{{.Values.outer | title }} {{.Values.inner | title}}")},
			{Name: "templates/test2", ModTime: modTime, Data: []byte("{{.Values.global.callme | lower }}")},
			{Name: "templates/test3", ModTime: modTime, Data: []byte("{{.noValue}}")},
			{Name: "templates/test4", ModTime: modTime, Data: []byte("{{toJson .Values}}")},
			{Name: "templates/test5", ModTime: modTime, Data: []byte("{{getHostByName \"helm.sh\"}}")},
		},
		Values: map[string]any{"outer": "DEFAULT", "inner": "DEFAULT"},
	}

	vals := map[string]any{
		"Values": map[string]any{
			"outer": "spouter",
			"inner": "inn",
			"global": map[string]any{
				"callme": "Ishmael",
			},
		},
	}

	v, err := util.CoalesceValues(c, vals)
	require.NoError(t, err, "Failed to coalesce values")
	out, err := Render(c, v)
	require.NoError(t, err, "Failed to render templates")

	expect := map[string]string{
		"moby/templates/test1": "Spouter Inn",
		"moby/templates/test2": "ishmael",
		"moby/templates/test3": "",
		"moby/templates/test4": `{"global":{"callme":"Ishmael"},"inner":"inn","outer":"spouter"}`,
		"moby/templates/test5": "",
	}

	for name, data := range expect {
		assert.Equal(t, data, out[name], "Expected %q, got %q", data, out[name])
	}
}

func TestRenderRefsOrdering(t *testing.T) {
	modTime := time.Now()

	parentChart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "parent",
			Version: "1.2.3",
		},
		Templates: []*common.File{
			{Name: "templates/_helpers.tpl", ModTime: modTime, Data: []byte(`{{- define "test" -}}parent value{{- end -}}`)},
			{Name: "templates/test.yaml", ModTime: modTime, Data: []byte(`{{ tpl "{{ include \"test\" . }}" . }}`)},
		},
	}
	childChart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "child",
			Version: "1.2.3",
		},
		Templates: []*common.File{
			{Name: "templates/_helpers.tpl", ModTime: modTime, Data: []byte(`{{- define "test" -}}child value{{- end -}}`)},
		},
	}
	parentChart.AddDependency(childChart)

	expect := map[string]string{
		"parent/templates/test.yaml": "parent value",
	}

	for i := range 100 {
		out, err := Render(parentChart, common.Values{})
		require.NoError(t, err, "Failed to render templates")

		for name, data := range expect {
			require.Equal(t, data, out[name], "Expected %q, got %q (iteration %d)", data, out[name], i+1)
		}
	}
}

func TestRenderInternals(t *testing.T) {
	// Test the internals of the rendering tool.

	vals := common.Values{"Name": "one", "Value": "two"}
	tpls := map[string]renderable{
		"one": {tpl: `Hello {{title .Name}}`, vals: vals},
		"two": {tpl: `Goodbye {{upper .Value}}`, vals: vals},
		// Test whether a template can reliably reference another template
		// without regard for ordering.
		"three": {tpl: `{{template "two" dict "Value" "three"}}`, vals: vals},
	}

	out, err := new(Engine).render(t.Context(), tpls)

	require.NoError(t, err, "Failed template rendering")
	require.Len(t, out, 3, "Expected 3 templates, got %d", len(out))
	assert.Equal(t, "Hello One", out["one"])
	assert.Equal(t, "Goodbye TWO", out["two"])
	assert.Equal(t, "Goodbye THREE", out["three"])
}

func TestRenderWithDNS(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "moby",
			Version: "1.2.3",
		},
		Templates: []*common.File{
			{Name: "templates/test1", ModTime: time.Now(), Data: []byte("{{getHostByName \"helm.sh\"}}")},
		},
		Values: map[string]any{},
	}

	vals := map[string]any{
		"Values": map[string]any{},
	}

	v, err := util.CoalesceValues(c, vals)
	require.NoError(t, err, "Failed to coalesce values")

	var e Engine
	e.EnableDNS = true
	out, err := e.Render(c, v)
	require.NoError(t, err, "Failed to render templates")

	for _, val := range c.Templates {
		fp := path.Join("moby", val.Name)
		assert.NotEmpty(t, out[fp], "Expected IP address, got %q", out[fp])
	}
}

type kindProps struct {
	shouldErr  error
	gvr        schema.GroupVersionResource
	namespaced bool
}

type testClientProvider struct {
	t       *testing.T
	scheme  map[string]kindProps
	objects []runtime.Object
}

func (p *testClientProvider) GetClientFor(apiVersion, kind string) (dynamic.NamespaceableResourceInterface, bool, error) {
	props := p.scheme[path.Join(apiVersion, kind)]
	if props.shouldErr != nil {
		return nil, false, props.shouldErr
	}
	return fake.NewSimpleDynamicClient(runtime.NewScheme(), p.objects...).Resource(props.gvr), props.namespaced, nil
}

var _ ClientProvider = &testClientProvider{}

// makeUnstructured is a convenience function for single-line creation of Unstructured objects.
func makeUnstructured(apiVersion, kind, name, namespace string) *unstructured.Unstructured {
	ret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name": name,
		},
	}}
	if namespace != "" {
		ret.Object["metadata"].(map[string]any)["namespace"] = namespace
	}
	return ret
}

func TestRenderWithClientProvider(t *testing.T) {
	provider := &testClientProvider{
		t: t,
		scheme: map[string]kindProps{
			"v1/Namespace": {
				gvr: schema.GroupVersionResource{
					Version:  "v1",
					Resource: "namespaces",
				},
			},
			"v1/Pod": {
				gvr: schema.GroupVersionResource{
					Version:  "v1",
					Resource: "pods",
				},
				namespaced: true,
			},
		},
		objects: []runtime.Object{
			makeUnstructured("v1", "Namespace", "default", ""),
			makeUnstructured("v1", "Pod", "pod1", "default"),
			makeUnstructured("v1", "Pod", "pod2", "ns1"),
			makeUnstructured("v1", "Pod", "pod3", "ns1"),
		},
	}

	type testCase struct {
		template string
		output   string
	}
	cases := map[string]testCase{
		"ns-single": {
			template: `{{ (lookup "v1" "Namespace" "" "default").metadata.name }}`,
			output:   "default",
		},
		"ns-list": {
			template: `{{ (lookup "v1" "Namespace" "" "").items | len }}`,
			output:   "1",
		},
		"ns-missing": {
			template: `{{ (lookup "v1" "Namespace" "" "absent") }}`,
			output:   "map[]",
		},
		"pod-single": {
			template: `{{ (lookup "v1" "Pod" "default" "pod1").metadata.name }}`,
			output:   "pod1",
		},
		"pod-list": {
			template: `{{ (lookup "v1" "Pod" "ns1" "").items | len }}`,
			output:   "2",
		},
		"pod-all": {
			template: `{{ (lookup "v1" "Pod" "" "").items | len }}`,
			output:   "3",
		},
		"pod-missing": {
			template: `{{ (lookup "v1" "Pod" "" "ns2") }}`,
			output:   "map[]",
		},
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "moby",
			Version: "1.2.3",
		},
		Values: map[string]any{},
	}

	modTime := time.Now()
	for name, exp := range cases {
		c.Templates = append(c.Templates, &common.File{
			Name:    path.Join("templates", name),
			ModTime: modTime,
			Data:    []byte(exp.template),
		})
	}

	vals := map[string]any{
		"Values": map[string]any{},
	}

	v, err := util.CoalesceValues(c, vals)
	require.NoError(t, err, "Failed to coalesce values")

	out, err := RenderWithClientProvider(c, v, provider)
	require.NoError(t, err, "Failed to render templates")

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			key := path.Join("moby/templates", name)
			assert.Equal(t, want.output, out[key], "Expected %q, got %q", want, out[key])
		})
	}
}

func TestRenderWithClientProvider_error(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "moby",
			Version: "1.2.3",
		},
		Templates: []*common.File{
			{Name: "templates/error", ModTime: time.Now(), Data: []byte(`{{ lookup "v1" "Error" "" "" }}`)},
		},
		Values: map[string]any{},
	}

	vals := map[string]any{
		"Values": map[string]any{},
	}

	v, err := util.CoalesceValues(c, vals)
	require.NoError(t, err, "Failed to coalesce values")

	provider := &testClientProvider{
		t: t,
		scheme: map[string]kindProps{
			"v1/Error": {
				shouldErr: errors.New("kaboom"),
			},
		},
	}
	_, err = RenderWithClientProvider(c, v, provider)
	assert.ErrorContainsf(t, err, "kaboom", "Expected error from client provider when rendering")
}

func TestParallelRenderInternals(t *testing.T) {
	// Make sure that we can use one Engine to run parallel template renders.
	e := new(Engine)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			tt := fmt.Sprintf("expect-%d", i)
			tpls := map[string]renderable{
				"t": {
					tpl:  `{{.val}}`,
					vals: map[string]any{"val": tt},
				},
			}
			out, err := e.render(t.Context(), tpls)
			assert.NoError(t, err, "Failed to render %s", tt)
			assert.Equal(t, tt, out["t"], "Expected %q, got %q", tt, out["t"])
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestParseErrors(t *testing.T) {
	vals := common.Values{"Values": map[string]any{}}

	tplsUndefinedFunction := map[string]renderable{
		"undefined_function": {tpl: `{{foo}}`, vals: vals},
	}
	_, err := new(Engine).render(t.Context(), tplsUndefinedFunction)
	require.Error(t, err, "Expected failures while rendering")
	assert.EqualError(t, err, `parse error at (undefined_function:1): function "foo" not defined`)
}

func TestExecErrors(t *testing.T) {
	vals := common.Values{"Values": map[string]any{}}
	cases := []struct {
		name     string
		tpls     map[string]renderable
		expected string
	}{
		{
			name: "MissingRequired",
			tpls: map[string]renderable{
				"missing_required": {tpl: `{{required "foo is required" .Values.foo}}`, vals: vals},
			},
			expected: `execution error at (missing_required:1:2): foo is required`,
		},
		{
			name: "MissingRequiredWithColons",
			tpls: map[string]renderable{
				"missing_required_with_colons": {tpl: `{{required ":this: message: has many: colons:" .Values.foo}}`, vals: vals},
			},
			expected: `execution error at (missing_required_with_colons:1:2): :this: message: has many: colons:`,
		},
		{
			name: "Issue6044",
			tpls: map[string]renderable{
				"issue6044": {
					vals: vals,
					tpl: `{{ $someEmptyValue := "" }}
{{ $myvar := "abc" }}
{{- required (printf "%s: something is missing" $myvar) $someEmptyValue | repeat 0 }}`,
				},
			},
			expected: `execution error at (issue6044:3:4): abc: something is missing`,
		},
		{
			name: "MissingRequiredWithNewlines",
			tpls: map[string]renderable{
				"issue9981": {tpl: `{{required "foo is required\nmore info after the break" .Values.foo}}`, vals: vals},
			},
			expected: `execution error at (issue9981:1:2): foo is required
more info after the break`,
		},
		{
			name: "FailWithNewlines",
			tpls: map[string]renderable{
				"issue9981": {tpl: `{{fail "something is wrong\nlinebreak"}}`, vals: vals},
			},
			expected: `execution error at (issue9981:1:2): something is wrong
linebreak`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := new(Engine).render(t.Context(), tt.tpls)
			require.Error(t, err, "Expected failures while rendering")
			assert.EqualError(t, err, tt.expected)
		})
	}
}

func TestFailErrors(t *testing.T) {
	vals := common.Values{"Values": map[string]any{}}

	failtpl := `All your base are belong to us{{ fail "This is an error" }}`
	tplsFailed := map[string]renderable{
		"failtpl": {tpl: failtpl, vals: vals},
	}
	_, err := new(Engine).render(t.Context(), tplsFailed)
	require.Error(t, err, "Expected failures while rendering")
	expected := `execution error at (failtpl:1:33): This is an error`
	require.EqualError(t, err, expected)

	var e Engine
	e.LintMode = true
	out, err := e.render(t.Context(), tplsFailed)
	require.NoError(t, err)
	assert.Equal(t, "All your base are belong to us", out["failtpl"])
}

func TestAllTemplates(t *testing.T) {
	modTime := time.Now()
	ch1 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "ch1"},
		Templates: []*common.File{
			{Name: "templates/foo", ModTime: modTime, Data: []byte("foo")},
			{Name: "templates/bar", ModTime: modTime, Data: []byte("bar")},
		},
	}
	dep1 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "laboratory mice"},
		Templates: []*common.File{
			{Name: "templates/pinky", ModTime: modTime, Data: []byte("pinky")},
			{Name: "templates/brain", ModTime: modTime, Data: []byte("brain")},
		},
	}
	ch1.AddDependency(dep1)

	dep2 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "same thing we do every night"},
		Templates: []*common.File{
			{Name: "templates/innermost", ModTime: modTime, Data: []byte("innermost")},
		},
	}
	dep1.AddDependency(dep2)

	tpls := allTemplates(ch1, common.Values{})
	assert.Len(t, tpls, 5, "Expected 5 charts, got %d", len(tpls))
}

func TestChartValuesContainsIsRoot(t *testing.T) {
	modTime := time.Now()
	ch1 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "parent"},
		Templates: []*common.File{
			{Name: "templates/isroot", ModTime: modTime, Data: []byte("{{.Chart.IsRoot}}")},
		},
	}
	dep1 := &chart.Chart{
		Metadata: &chart.Metadata{Name: "child"},
		Templates: []*common.File{
			{Name: "templates/isroot", ModTime: modTime, Data: []byte("{{.Chart.IsRoot}}")},
		},
	}
	ch1.AddDependency(dep1)

	out, err := Render(ch1, common.Values{})
	require.NoError(t, err, "failed to render templates")
	expects := map[string]string{
		"parent/charts/child/templates/isroot": "false",
		"parent/templates/isroot":              "true",
	}
	for file, expect := range expects {
		assert.Equal(t, expect, out[file], "Expected %q, got %q", expect, out[file])
	}
}

func TestRenderDependency(t *testing.T) {
	deptpl := `{{define "myblock"}}World{{end}}`
	toptpl := `Hello {{template "myblock"}}`
	modTime := time.Now()
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "outerchart"},
		Templates: []*common.File{
			{Name: "templates/outer", ModTime: modTime, Data: []byte(toptpl)},
		},
	}
	ch.AddDependency(&chart.Chart{
		Metadata: &chart.Metadata{Name: "innerchart"},
		Templates: []*common.File{
			{Name: "templates/inner", ModTime: modTime, Data: []byte(deptpl)},
		},
	})

	out, err := Render(ch, map[string]any{})
	require.NoError(t, err, "failed to render chart")

	assert.Len(t, out, 2, "Expected 2, got %d", len(out))
	assert.Equal(t, "Hello World", out["outerchart/templates/outer"])
}

func TestRenderNestedValues(t *testing.T) {
	innerpath := "templates/inner.tpl"
	outerpath := "templates/outer.tpl"
	// Ensure namespacing rules are working.
	deepestpath := "templates/inner.tpl"
	checkrelease := "templates/release.tpl"
	// Ensure subcharts scopes are working.
	subchartspath := "templates/subcharts.tpl"

	modTime := time.Now()
	deepest := &chart.Chart{
		Metadata: &chart.Metadata{Name: "deepest"},
		Templates: []*common.File{
			{Name: deepestpath, ModTime: modTime, Data: []byte(`And this same {{.Values.what}} that smiles {{.Values.global.when}}`)},
			{Name: checkrelease, ModTime: modTime, Data: []byte(`Tomorrow will be {{default "happy" .Release.Name }}`)},
		},
		Values: map[string]any{"what": "milkshake", "where": "here"},
	}

	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "herrick"},
		Templates: []*common.File{
			{Name: innerpath, ModTime: modTime, Data: []byte(`Old {{.Values.who}} is still a-flyin'`)},
		},
		Values: map[string]any{"who": "Robert", "what": "glasses"},
	}
	inner.AddDependency(deepest)

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "top"},
		Templates: []*common.File{
			{Name: outerpath, ModTime: modTime, Data: []byte(`Gather ye {{.Values.what}} while ye may`)},
			{Name: subchartspath, ModTime: modTime, Data: []byte(`The glorious Lamp of {{.Subcharts.herrick.Subcharts.deepest.Values.where}}, the {{.Subcharts.herrick.Values.what}}`)},
		},
		Values: map[string]any{
			"what": "stinkweed",
			"who":  "me",
			"herrick": map[string]any{
				"who":  "time",
				"what": "Sun",
			},
		},
	}
	outer.AddDependency(inner)

	injValues := map[string]any{
		"what": "rosebuds",
		"herrick": map[string]any{
			"deepest": map[string]any{
				"what":  "flower",
				"where": "Heaven",
			},
		},
		"global": map[string]any{
			"when": "to-day",
		},
	}

	tmp, err := util.CoalesceValues(outer, injValues)
	require.NoError(t, err, "Failed to coalesce values")

	inject := common.Values{
		"Values": tmp,
		"Chart":  outer.Metadata,
		"Release": common.Values{
			"Name": "dyin",
		},
	}

	t.Logf("Calculated values: %v", inject)

	out, err := Render(outer, inject)
	require.NoError(t, err, "failed to render templates")

	fullouterpath := "top/" + outerpath
	assert.Equal(t, "Gather ye rosebuds while ye may", out[fullouterpath], "Unexpected outer: %q", out[fullouterpath])

	fullinnerpath := "top/charts/herrick/" + innerpath
	assert.Equal(t, "Old time is still a-flyin'", out[fullinnerpath], "Unexpected inner: %q", out[fullinnerpath])

	fulldeepestpath := "top/charts/herrick/charts/deepest/" + deepestpath
	assert.Equal(t, "And this same flower that smiles to-day", out[fulldeepestpath], "Unexpected deepest: %q", out[fulldeepestpath])

	fullcheckrelease := "top/charts/herrick/charts/deepest/" + checkrelease
	assert.Equal(t, "Tomorrow will be dyin", out[fullcheckrelease], "Unexpected release: %q", out[fullcheckrelease])

	fullchecksubcharts := "top/" + subchartspath
	assert.Equal(t, "The glorious Lamp of Heaven, the Sun", out[fullchecksubcharts], "Unexpected subcharts: %q", out[fullchecksubcharts])
}

func TestRenderBuiltinValues(t *testing.T) {
	modTime := time.Now()
	inner := &chart.Chart{
		Metadata: &chart.Metadata{Name: "Latium", APIVersion: chart.APIVersionV2},
		Templates: []*common.File{
			{Name: "templates/Lavinia", ModTime: modTime, Data: []byte(`{{.Template.Name}}{{.Chart.Name}}{{.Release.Name}}`)},
			{Name: "templates/From", ModTime: modTime, Data: []byte(`{{.Files.author | printf "%s"}} {{.Files.Get "book/title.txt"}}`)},
		},
		Files: []*common.File{
			{Name: "author", ModTime: modTime, Data: []byte("Virgil")},
			{Name: "book/title.txt", ModTime: modTime, Data: []byte("Aeneid")},
		},
	}

	outer := &chart.Chart{
		Metadata: &chart.Metadata{Name: "Troy", APIVersion: chart.APIVersionV2},
		Templates: []*common.File{
			{Name: "templates/Aeneas", ModTime: modTime, Data: []byte(`{{.Template.Name}}{{.Chart.Name}}{{.Release.Name}}`)},
			{Name: "templates/Amata", ModTime: modTime, Data: []byte(`{{.Subcharts.Latium.Chart.Name}} {{.Subcharts.Latium.Files.author | printf "%s"}}`)},
		},
	}
	outer.AddDependency(inner)

	inject := common.Values{
		"Values": "",
		"Chart":  outer.Metadata,
		"Release": common.Values{
			"Name": "Aeneid",
		},
	}

	t.Logf("Calculated values: %v", outer)

	out, err := Render(outer, inject)
	require.NoError(t, err, "failed to render templates")

	expects := map[string]string{
		"Troy/charts/Latium/templates/Lavinia": "Troy/charts/Latium/templates/LaviniaLatiumAeneid",
		"Troy/templates/Aeneas":                "Troy/templates/AeneasTroyAeneid",
		"Troy/templates/Amata":                 "Latium Virgil",
		"Troy/charts/Latium/templates/From":    "Virgil Aeneid",
	}
	for file, expect := range expects {
		assert.Equal(t, expect, out[file], "Expected %q, got %q", expect, out[file])
	}
}

func TestAlterFuncMap_include(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "conrad"},
		Templates: []*common.File{
			{Name: "templates/quote", ModTime: modTime, Data: []byte(`{{include "conrad/templates/_partial" . | indent 2}} dead.`)},
			{Name: "templates/_partial", ModTime: modTime, Data: []byte(`{{.Release.Name}} - he`)},
		},
	}

	// Check nested reference in include FuncMap
	d := &chart.Chart{
		Metadata: &chart.Metadata{Name: "nested"},
		Templates: []*common.File{
			{Name: "templates/quote", ModTime: modTime, Data: []byte(`{{include "nested/templates/quote" . | indent 2}} dead.`)},
			{Name: "templates/_partial", ModTime: modTime, Data: []byte(`{{.Release.Name}} - he`)},
		},
	}

	v := common.Values{
		"Values": "",
		"Chart":  c.Metadata,
		"Release": common.Values{
			"Name": "Mistah Kurtz",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)
	assert.Equal(t, "  Mistah Kurtz - he dead.", out["conrad/templates/quote"])

	_, err = Render(d, v)
	expectErrName := "nested/templates/quote"
	assert.Error(t, err, "Expected err of nested reference name: %v", expectErrName)
}

func TestAlterFuncMap_require(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "conan"},
		Templates: []*common.File{
			{Name: "templates/quote", ModTime: modTime, Data: []byte(`All your base are belong to {{ required "A valid 'who' is required" .Values.who }}`)},
			{Name: "templates/bases", ModTime: modTime, Data: []byte(`All {{ required "A valid 'bases' is required" .Values.bases }} of them!`)},
		},
	}

	v := common.Values{
		"Values": common.Values{
			"who":   "us",
			"bases": 2,
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "That 90s meme",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)

	assert.Equal(t, "All your base are belong to us", out["conan/templates/quote"])
	assert.Equal(t, "All 2 of them!", out["conan/templates/bases"])

	// test required without passing in needed values with lint mode on
	// verifies lint replaces required with an empty string (should not fail)
	lintValues := common.Values{
		"Values": common.Values{
			"who": "us",
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "That 90s meme",
		},
	}
	var e Engine
	e.LintMode = true

	out, err = e.Render(c, lintValues)
	require.NoError(t, err)
	assert.Equal(t, "All your base are belong to us", out["conan/templates/quote"])
	assert.Equal(t, "All  of them!", out["conan/templates/bases"])
}

func TestAlterFuncMap_tpl(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplFunction"},
		Templates: []*common.File{
			{Name: "templates/base", ModTime: time.Now(), Data: []byte(`Evaluate tpl {{tpl "Value: {{ .Values.value}}" .}}`)},
		},
	}

	v := common.Values{
		"Values": common.Values{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)
	assert.Equal(t, "Evaluate tpl Value: myvalue", out["TplFunction/templates/base"])
}

func TestAlterFuncMap_tplfunc(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplFunction"},
		Templates: []*common.File{
			{Name: "templates/base", ModTime: time.Now(), Data: []byte(`Evaluate tpl {{tpl "Value: {{ .Values.value | quote}}" .}}`)},
		},
	}

	v := common.Values{
		"Values": common.Values{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)
	assert.Equal(t, "Evaluate tpl Value: \"myvalue\"", out["TplFunction/templates/base"])
}

func TestAlterFuncMap_tplinclude(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplFunction"},
		Templates: []*common.File{
			{Name: "templates/base", ModTime: modTime, Data: []byte(`{{ tpl "{{include ` + "`" + `TplFunction/templates/_partial` + "`" + ` .  | quote }}" .}}`)},
			{Name: "templates/_partial", ModTime: modTime, Data: []byte(`{{.Template.Name}}`)},
		},
	}
	v := common.Values{
		"Values": common.Values{
			"value": "myvalue",
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)
	assert.Equal(t, "\"TplFunction/templates/base\"", out["TplFunction/templates/base"])
}

func TestRenderRecursionLimit(t *testing.T) {
	modTime := time.Now()

	// endless recursion should produce an error
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "bad"},
		Templates: []*common.File{
			{Name: "templates/base", ModTime: modTime, Data: []byte(`{{include "recursion" . }}`)},
			{Name: "templates/recursion", ModTime: modTime, Data: []byte(`{{define "recursion"}}{{include "recursion" . }}{{end}}`)},
		},
	}
	v := common.Values{
		"Values": "",
		"Chart":  c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}
	expectErr := "rendering template has a nested reference name: recursion: unable to execute template"

	_, err := Render(c, v)
	require.Error(t, err)
	assert.True(t, strings.HasSuffix(err.Error(), expectErr), "Expected err with suffix: %s", expectErr)

	// calling the same function many times is ok
	times := 4000
	phrase := "All work and no play makes Jack a dull boy"
	printFunc := `{{define "overlook"}}{{printf "` + phrase + `\n"}}{{end}}`
	var repeatedIncl strings.Builder
	for range times {
		repeatedIncl.WriteString(`{{include "overlook" . }}`)
	}

	d := &chart.Chart{
		Metadata: &chart.Metadata{Name: "overlook"},
		Templates: []*common.File{
			{Name: "templates/quote", ModTime: modTime, Data: []byte(repeatedIncl.String())},
			{Name: "templates/_function", ModTime: modTime, Data: []byte(printFunc)},
		},
	}

	out, err := Render(d, v)
	require.NoError(t, err)

	var expect string
	var expectSb1062 strings.Builder
	for range times {
		expectSb1062.WriteString(phrase + "\n")
	}
	expect += expectSb1062.String()
	assert.Equal(t, expect, out["overlook/templates/quote"])
}

func TestRenderLoadTemplateForTplFromFile(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplLoadFromFile"},
		Templates: []*common.File{
			{Name: "templates/base", ModTime: modTime, Data: []byte(`{{ tpl (.Files.Get .Values.filename) . }}`)},
			{Name: "templates/_function", ModTime: modTime, Data: []byte(`{{define "test-function"}}test-function{{end}}`)},
		},
		Files: []*common.File{
			{Name: "test", ModTime: modTime, Data: []byte(`{{ tpl (.Files.Get .Values.filename2) .}}`)},
			{Name: "test2", ModTime: modTime, Data: []byte(`{{include "test-function" .}}{{define "nested-define"}}nested-define-content{{end}} {{include "nested-define" .}}`)},
		},
	}

	v := common.Values{
		"Values": common.Values{
			"filename":  "test",
			"filename2": "test2",
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)
	require.Equal(t, "test-function nested-define-content", out["TplLoadFromFile/templates/base"])
}

func TestRenderTplEmpty(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplEmpty"},
		Templates: []*common.File{
			{Name: "templates/empty-string", ModTime: modTime, Data: []byte(`{{tpl "" .}}`)},
			{Name: "templates/empty-action", ModTime: modTime, Data: []byte(`{{tpl "{{ \"\"}}" .}}`)},
			{Name: "templates/only-defines", ModTime: modTime, Data: []byte(`{{tpl "{{define \"not-invoked\"}}not-rendered{{end}}" .}}`)},
		},
	}
	v := common.Values{
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)

	expects := map[string]string{
		"TplEmpty/templates/empty-string": "",
		"TplEmpty/templates/empty-action": "",
		"TplEmpty/templates/only-defines": "",
	}
	for file, expect := range expects {
		assert.Equal(t, expect, out[file], "Expected %q, got %q", expect, out[file])
	}
}

func TestRenderTplTemplateNames(t *testing.T) {
	modTime := time.Now()
	// .Template.BasePath and .Name make it through
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplTemplateNames"},
		Templates: []*common.File{
			{Name: "templates/default-basepath", ModTime: modTime, Data: []byte(`{{tpl "{{ .Template.BasePath }}" .}}`)},
			{Name: "templates/default-name", ModTime: modTime, Data: []byte(`{{tpl "{{ .Template.Name }}" .}}`)},
			{Name: "templates/modified-basepath", ModTime: modTime, Data: []byte(`{{tpl "{{ .Template.BasePath }}" .Values.dot}}`)},
			{Name: "templates/modified-name", ModTime: modTime, Data: []byte(`{{tpl "{{ .Template.Name }}" .Values.dot}}`)},
			{Name: "templates/modified-field", ModTime: modTime, Data: []byte(`{{tpl "{{ .Template.Field }}" .Values.dot}}`)},
		},
	}
	v := common.Values{
		"Values": common.Values{
			"dot": common.Values{
				"Template": common.Values{
					"BasePath": "path/to/template",
					"Name":     "name-of-template",
					"Field":    "extra-field",
				},
			},
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)

	expects := map[string]string{
		"TplTemplateNames/templates/default-basepath":  "TplTemplateNames/templates",
		"TplTemplateNames/templates/default-name":      "TplTemplateNames/templates/default-name",
		"TplTemplateNames/templates/modified-basepath": "path/to/template",
		"TplTemplateNames/templates/modified-name":     "name-of-template",
		"TplTemplateNames/templates/modified-field":    "extra-field",
	}
	for file, expect := range expects {
		assert.Equal(t, expect, out[file], "Expected %q, got %q", expect, out[file])
	}
}

func TestRenderTplRedefines(t *testing.T) {
	modTime := time.Now()
	// Redefining a template inside 'tpl' does not affect the outer definition
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplRedefines"},
		Templates: []*common.File{
			{Name: "templates/_partials", ModTime: modTime, Data: []byte(`{{define "partial"}}original-in-partial{{end}}`)},
			{Name: "templates/partial", ModTime: modTime, Data: []byte(
				`before: {{include "partial" .}}\n{{tpl .Values.partialText .}}\nafter: {{include "partial" .}}`,
			)},
			{Name: "templates/manifest", Data: []byte(
				`{{define "manifest"}}original-in-manifest{{end}}` +
					`before: {{include "manifest" .}}\n{{tpl .Values.manifestText .}}\nafter: {{include "manifest" .}}`,
			)},
			{Name: "templates/manifest-only", Data: []byte(
				`{{define "manifest-only"}}only-in-manifest{{end}}` +
					`before: {{include "manifest-only" .}}\n{{tpl .Values.manifestOnlyText .}}\nafter: {{include "manifest-only" .}}`,
			)},
			{Name: "templates/nested", Data: []byte(
				`{{define "nested"}}original-in-manifest{{end}}` +
					`{{define "nested-outer"}}original-outer-in-manifest{{end}}` +
					`before: {{include "nested" .}} {{include "nested-outer" .}}\n` +
					`{{tpl .Values.nestedText .}}\n` +
					`after: {{include "nested" .}} {{include "nested-outer" .}}`,
			)},
		},
	}
	v := common.Values{
		"Values": common.Values{
			"partialText":      `{{define "partial"}}redefined-in-tpl{{end}}tpl: {{include "partial" .}}`,
			"manifestText":     `{{define "manifest"}}redefined-in-tpl{{end}}tpl: {{include "manifest" .}}`,
			"manifestOnlyText": `tpl: {{include "manifest-only" .}}`,
			"nestedText": `{{define "nested"}}redefined-in-tpl{{end}}` +
				`{{define "nested-outer"}}redefined-outer-in-tpl{{end}}` +
				`before-inner-tpl: {{include "nested" .}} {{include "nested-outer" . }}\n` +
				`{{tpl .Values.innerText .}}\n` +
				`after-inner-tpl: {{include "nested" .}} {{include "nested-outer" . }}`,
			"innerText": `{{define "nested"}}redefined-in-inner-tpl{{end}}inner-tpl: {{include "nested" .}} {{include "nested-outer" . }}`,
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)

	expects := map[string]string{
		"TplRedefines/templates/partial":       `before: original-in-partial\ntpl: redefined-in-tpl\nafter: original-in-partial`,
		"TplRedefines/templates/manifest":      `before: original-in-manifest\ntpl: redefined-in-tpl\nafter: original-in-manifest`,
		"TplRedefines/templates/manifest-only": `before: only-in-manifest\ntpl: only-in-manifest\nafter: only-in-manifest`,
		"TplRedefines/templates/nested": `before: original-in-manifest original-outer-in-manifest\n` +
			`before-inner-tpl: redefined-in-tpl redefined-outer-in-tpl\n` +
			`inner-tpl: redefined-in-inner-tpl redefined-outer-in-tpl\n` +
			`after-inner-tpl: redefined-in-tpl redefined-outer-in-tpl\n` +
			`after: original-in-manifest original-outer-in-manifest`,
	}
	for file, expect := range expects {
		assert.Equal(t, expect, out[file], "Expected %q, got %q", expect, out[file])
	}
}

func TestRenderTplMissingKey(t *testing.T) {
	// Rendering a missing key results in empty/zero output.
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplMissingKey"},
		Templates: []*common.File{
			{Name: "templates/manifest", ModTime: time.Now(), Data: []byte(
				`missingValue: {{tpl "{{.Values.noSuchKey}}" .}}`,
			)},
		},
	}
	v := common.Values{
		"Values": common.Values{},
		"Chart":  c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	out, err := Render(c, v)
	require.NoError(t, err)

	expects := map[string]string{
		"TplMissingKey/templates/manifest": `missingValue: `,
	}
	for file, expect := range expects {
		assert.Equal(t, expect, out[file], "Expected %q, got %q", expect, out[file])
	}
}

func TestRenderTplMissingKeyString(t *testing.T) {
	// Rendering a missing key results in error
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "TplMissingKeyStrict"},
		Templates: []*common.File{
			{Name: "templates/manifest", ModTime: time.Now(), Data: []byte(
				`missingValue: {{tpl "{{.Values.noSuchKey}}" .}}`,
			)},
		},
	}
	v := common.Values{
		"Values": common.Values{},
		"Chart":  c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	e := new(Engine)
	e.Strict = true

	_, err := e.Render(c, v)
	require.Error(t, err)
	assert.ErrorContains(t, err, "noSuchKey")
}

func TestNestedHelpersProducesMultilineStacktrace(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "NestedHelperFunctions"},
		Templates: []*common.File{
			{Name: "templates/svc.yaml", ModTime: modTime, Data: []byte(
				`name: {{ include "nested_helper.name" . }}`,
			)},
			{Name: "templates/_helpers_1.tpl", ModTime: modTime, Data: []byte(
				`{{- define "nested_helper.name" -}}{{- include "common.names.get_name" . -}}{{- end -}}`,
			)},
			{Name: "charts/common/templates/_helpers_2.tpl", ModTime: modTime, Data: []byte(
				`{{- define "common.names.get_name" -}}{{- .Values.nonexistant.key | trunc 63 | trimSuffix "-" -}}{{- end -}}`,
			)},
		},
	}

	expectedErrorMessage := `NestedHelperFunctions/templates/svc.yaml:1:9
  executing "NestedHelperFunctions/templates/svc.yaml" at <include "nested_helper.name" .>:
    error calling include:
NestedHelperFunctions/templates/_helpers_1.tpl:1:39
  executing "nested_helper.name" at <include "common.names.get_name" .>:
    error calling include:
NestedHelperFunctions/charts/common/templates/_helpers_2.tpl:1:49
  executing "common.names.get_name" at <.Values.nonexistant.key>:
    nil pointer evaluating interface {}.key`

	v := common.Values{}

	val, _ := util.CoalesceValues(c, v)
	vals := map[string]any{
		"Values": val.AsMap(),
	}
	_, err := Render(c, vals)

	require.Error(t, err)
	assert.EqualError(t, err, expectedErrorMessage)
}

func TestMultilineNoTemplateAssociatedError(t *testing.T) {
	modTime := time.Now()
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "multiline"},
		Templates: []*common.File{
			{Name: "templates/svc.yaml", ModTime: modTime, Data: []byte(
				`name: {{ include "nested_helper.name" . }}`,
			)},
			{Name: "templates/test.yaml", ModTime: modTime, Data: []byte(
				`{{ toYaml .Values }}`,
			)},
			{Name: "charts/common/templates/_helpers_2.tpl", ModTime: modTime, Data: []byte(
				`{{ toYaml .Values }}`,
			)},
		},
	}

	v := common.Values{}

	val, _ := util.CoalesceValues(c, v)
	vals := map[string]any{
		"Values": val.AsMap(),
	}
	_, err := Render(c, vals)

	require.Error(t, err)
	assert.EqualError(t, err, `multiline/templates/svc.yaml:1:9
  executing "multiline/templates/svc.yaml" at <include "nested_helper.name" .>:
    error calling include:
template: no template "nested_helper.name" associated with template "gotpl"`)
}

func TestRenderCustomTemplateFuncs(t *testing.T) {
	modTime := time.Now()

	// Create a chart with two templates that use custom functions
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "CustomFunc"},
		Templates: []*common.File{
			{
				Name:    "templates/manifest",
				ModTime: modTime,
				Data:    []byte(`{{exclaim .Values.message}}`),
			},
			{
				Name:    "templates/override",
				ModTime: modTime,
				Data:    []byte(`{{ upper .Values.message }}`),
			},
		},
	}
	v := common.Values{
		"Values": common.Values{
			"message": "hello",
		},
		"Chart": c.Metadata,
		"Release": common.Values{
			"Name": "TestRelease",
		},
	}

	// Define a custom template function "exclaim" that appends "!!!" to a string and override "upper" function
	customFuncs := template.FuncMap{
		"exclaim": func(input string) string {
			return input + "!!!"
		},
		"upper": func(s string) string {
			return "custom:" + s
		},
	}

	// Create an engine instance and set the CustomTemplateFuncs.
	e := new(Engine)
	e.CustomTemplateFuncs = customFuncs

	// Render the chart.
	out, err := e.Render(c, v)
	require.NoError(t, err)

	// Expected output should be "hello!!!".
	rendered, ok := out["CustomFunc/templates/manifest"]
	require.True(t, ok)
	assert.Equal(t, "hello!!!", rendered)

	// Verify that the rendered template used the custom "upper" function.
	rendered, ok = out["CustomFunc/templates/override"]
	require.True(t, ok)
	assert.Equal(t, "custom:hello", rendered)
}

func TestTraceableError_SimpleForm(t *testing.T) {
	testStrings := []string{
		"function_not_found/templates/secret.yaml: error calling include",
	}
	for _, errString := range testStrings {
		trace, done := parseTemplateSimpleErrorString(errString)
		assert.True(t, done, "Expected parse to pass but did not")
		assert.Equal(t, "error calling include", trace.message, "Expected %q, got %q", errString, trace.message)
	}
}
func TestTraceableError_ExecutingForm(t *testing.T) {
	testStrings := [][]string{
		{"function_not_found/templates/secret.yaml:6:11: executing \"function_not_found/templates/secret.yaml\" at <include \"name\" .>: ", "function_not_found/templates/secret.yaml:6:11"},
		{"divide_by_zero/templates/secret.yaml:6:11: executing \"divide_by_zero/templates/secret.yaml\" at <include \"division\" .>: ", "divide_by_zero/templates/secret.yaml:6:11"},
	}
	for _, errTuple := range testStrings {
		errString := errTuple[0]
		expectedLocation := errTuple[1]
		trace, done := parseTemplateExecutingAtErrorType(errString)
		assert.True(t, done, "Expected parse to pass but did not")
		assert.Equal(t, expectedLocation, trace.location, "Expected %q, got %q", expectedLocation, trace.location)
	}
}

func TestTraceableError_NoTemplateForm(t *testing.T) {
	testStrings := []string{
		"no template \"common.names.get_name\" associated with template \"gotpl\"",
	}
	for _, errString := range testStrings {
		trace, done := parseTemplateNoTemplateError(errString, errString)
		assert.True(t, done, "Expected parse to pass but did not")
		assert.Equal(t, errString, trace.message, "Expected %q, got %q", errString, trace.message)
	}
}

// TestRenderSubchartDefaultNilNoStringify tests the full pipeline: subchart default
// nil values should not produce "%!s(<nil>)" in rendered template output.
// Regression test for the Bitnami common.secrets.key issue.
func TestRenderSubchartDefaultNilNoStringify(t *testing.T) {
	modTime := time.Now()

	// Subchart has a default with nil values
	subchart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "child"},
		Templates: []*common.File{
			{
				Name:    "templates/test.yaml",
				ModTime: modTime,
				Data:    []byte(`{{- if hasKey .Values.keyMapping "password" -}}{{- printf "subPath: %s" (index .Values.keyMapping "password") -}}{{- else -}}subPath: fallback{{- end -}}`),
			},
		},
		Values: map[string]any{
			"keyMapping": map[string]any{
				"password": nil, // nil in chart defaults
			},
		},
	}

	parent := &chart.Chart{
		Metadata: &chart.Metadata{Name: "parent"},
		Values:   map[string]any{},
	}
	parent.AddDependency(subchart)

	// Parent user values don't set keyMapping
	injValues := map[string]any{}

	tmp, err := util.CoalesceValues(parent, injValues)
	require.NoError(t, err, "Failed to coalesce values")

	inject := common.Values{
		"Values": tmp,
		"Chart":  parent.Metadata,
		"Release": common.Values{
			"Name": "test-release",
		},
	}

	out, err := Render(parent, inject)
	require.NoError(t, err, "Failed to render templates")

	rendered := out["parent/charts/child/templates/test.yaml"]
	assert.NotContains(t, rendered, "%!s(<nil>)", "Rendered output contains %%!s(<nil>), got: %q", rendered)
	assert.Equal(t, "subPath: fallback", rendered)
}
