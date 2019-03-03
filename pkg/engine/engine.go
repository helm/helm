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
	"path"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chartutil"
)

// Engine is an implementation of 'cmd/tiller/environment'.Engine that uses Go templates.
type Engine struct {
	// FuncMap contains the template functions that will be passed to each
	// render call. This may only be modified before the first call to Render.
	funcMap template.FuncMap
	// If strict is enabled, template rendering will fail if a template references
	// a value that was not passed in.
	Strict bool
}

// New creates a new Go template Engine instance.
//
// The FuncMap is initialized here. You may modify the FuncMap _prior to_ the
// first invocation of Render.
//
// The FuncMap sets all of the Sprig functions except for those that provide
// access to the underlying OS (env, expandenv).
func New() *Engine {
	return &Engine{funcMap: FuncMap()}
}

// FuncMap returns a mapping of all of the functions that Engine has.
//
// Because some functions are late-bound (e.g. contain context-sensitive
// data), the functions may not all perform identically outside of an
// Engine as they will inside of an Engine.
//
// Known late-bound functions:
//
//	- "include": This is late-bound in Engine.Render(). The version
//	   included in the FuncMap is a placeholder.
//      - "required": This is late-bound in Engine.Render(). The version
//	   included in the FuncMap is a placeholder.
//      - "tpl": This is late-bound in Engine.Render(). The version
//	   included in the FuncMap is a placeholder.
func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// Add some extra functionality
	extra := template.FuncMap{
		"toToml":   chartutil.ToTOML,
		"toYaml":   chartutil.ToYAML,
		"fromYaml": chartutil.FromYAML,
		"toJson":   chartutil.ToJSON,
		"fromJson": chartutil.FromJSON,

		// This is a placeholder for the "include" function, which is
		// late-bound to a template. By declaring it here, we preserve the
		// integrity of the linter.
		"include":  func(string, interface{}) string { return "not implemented" },
		"required": func(string, interface{}) interface{} { return "not implemented" },
		"tpl":      func(string, interface{}) interface{} { return "not implemented" },
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

// Render takes a chart, optional values, and value overrides, and attempts to render the Go templates.
//
// Render can be called repeatedly on the same engine.
//
// This will look in the chart's 'templates' data (e.g. the 'templates/' directory)
// and attempt to render the templates there using the values passed in.
//
// Values are scoped to their templates. A dependency template will not have
// access to the values set for its parent. If chart "foo" includes chart "bar",
// "bar" will not have access to the values for "foo".
//
// Values should be prepared with something like `chartutils.ReadValues`.
//
// Values are passed through the templates according to scope. If the top layer
// chart includes the chart foo, which includes the chart bar, the values map
// will be examined for a table called "foo". If "foo" is found in vals,
// that section of the values will be passed into the "foo" chart. And if that
// section contains a value named "bar", that value will be passed on to the
// bar chart during render time.
func (e *Engine) Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	// Render the charts
	tmap := allTemplates(chrt, values)
	return e.render(tmap)
}

// renderable is an object that can be rendered.
type renderable struct {
	// tpl is the current template.
	tpl string
	// vals are the values to be supplied to the template.
	vals chartutil.Values
	// namespace prefix to the templates of the current chart
	basePath string
}

// alterFuncMap takes the Engine's FuncMap and adds context-specific functions.
//
// The resulting FuncMap is only valid for the passed-in template.
func (e *Engine) alterFuncMap(t *template.Template, referenceTpls map[string]renderable) template.FuncMap {
	// Clone the func map because we are adding context-specific functions.
	funcMap := make(template.FuncMap)
	for k, v := range e.funcMap {
		funcMap[k] = v
	}

	// Add the 'include' function here so we can close over t.
	funcMap["include"] = func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		err := t.ExecuteTemplate(&buf, name, data)
		return buf.String(), err
	}

	// Add the 'required' function here
	funcMap["required"] = func(warn string, val interface{}) (interface{}, error) {
		if val == nil {
			return val, errors.Errorf(warn)
		} else if _, ok := val.(string); ok {
			if val == "" {
				return val, errors.Errorf(warn)
			}
		}
		return val, nil
	}

	// Add the 'tpl' function here
	funcMap["tpl"] = func(tpl string, vals chartutil.Values) (string, error) {
		basePath, err := vals.PathValue("Template.BasePath")
		if err != nil {
			return "", errors.Wrapf(err, "cannot retrieve Template.Basepath from values inside tpl function: %s", tpl)
		}

		r := renderable{
			tpl:      tpl,
			vals:     vals,
			basePath: basePath.(string),
		}

		templateName, err := vals.PathValue("Template.Name")
		if err != nil {
			return "", errors.Wrapf(err, "cannot retrieve Template.Name from values inside tpl function: %s", tpl)
		}

		templates := make(map[string]renderable)
		templates[templateName.(string)] = r

		result, err := e.renderWithReferences(templates, referenceTpls)
		if err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}
		return result[templateName.(string)], nil
	}

	return funcMap
}

// render takes a map of templates/values and renders them.
func (e *Engine) render(tpls map[string]renderable) (rendered map[string]string, err error) {
	return e.renderWithReferences(tpls, tpls)
}

// renderWithReferences takes a map of templates/values to render, and a map of
// templates which can be referenced within them.
func (e *Engine) renderWithReferences(tpls, referenceTpls map[string]renderable) (rendered map[string]string, err error) {

	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("rendering template failed: %v", r)
		}
	}()
	t := template.New("gotpl")
	if e.Strict {
		t.Option("missingkey=error")
	} else {
		// Not that zero will attempt to add default values for types it knows,
		// but will still emit <no value> for others. We mitigate that later.
		t.Option("missingkey=zero")
	}

	funcMap := e.alterFuncMap(t, referenceTpls)

	// We want to parse the templates in a predictable order. The order favors
	// higher-level (in file system) templates over deeply nested templates.
	keys := sortTemplates(tpls)

	files := []string{}

	for _, fname := range keys {
		r := tpls[fname]
		if _, err := t.New(fname).Funcs(funcMap).Parse(r.tpl); err != nil {
			return map[string]string{}, errors.Wrapf(err, "parse error in %q", fname)
		}
		files = append(files, fname)
	}

	// Adding the reference templates to the template context
	// so they can be referenced in the tpl function
	for fname, r := range referenceTpls {
		if t.Lookup(fname) == nil {
			if _, err := t.New(fname).Funcs(funcMap).Parse(r.tpl); err != nil {
				return map[string]string{}, errors.Wrapf(err, "parse error in %q", fname)
			}
		}
	}

	rendered = make(map[string]string, len(files))
	for _, file := range files {
		// Don't render partials. We don't care out the direct output of partials.
		// They are only included from other templates.
		if strings.HasPrefix(path.Base(file), "_") {
			continue
		}
		// At render time, add information about the template that is being rendered.
		vals := tpls[file].vals
		vals["Template"] = chartutil.Values{"Name": file, "BasePath": tpls[file].basePath}
		var buf strings.Builder
		if err := t.ExecuteTemplate(&buf, file, vals); err != nil {
			return map[string]string{}, errors.Wrapf(err, "render error in %q", file)
		}

		// Work around the issue where Go will emit "<no value>" even if Options(missing=zero)
		// is set. Since missing=error will never get here, we do not need to handle
		// the Strict case.
		f := &chart.File{
			Name: strings.Replace(file, "/templates", "/manifests", -1),
			Data: []byte(strings.Replace(buf.String(), "<no value>", "", -1)),
		}
		rendered[file] = string(f.Data)
		// if ch != nil {
		// 	ch.Files = append(ch.Files, f)
		// }
	}

	return rendered, nil
}

func sortTemplates(tpls map[string]renderable) []string {
	keys := make([]string, len(tpls))
	i := 0
	for key := range tpls {
		keys[i] = key
		i++
	}
	sort.Sort(sort.Reverse(byPathLen(keys)))
	return keys
}

type byPathLen []string

func (p byPathLen) Len() int      { return len(p) }
func (p byPathLen) Swap(i, j int) { p[j], p[i] = p[i], p[j] }
func (p byPathLen) Less(i, j int) bool {
	a, b := p[i], p[j]
	ca, cb := strings.Count(a, "/"), strings.Count(b, "/")
	if ca == cb {
		return strings.Compare(a, b) == -1
	}
	return ca < cb
}

// allTemplates returns all templates for a chart and its dependencies.
//
// As it goes, it also prepares the values in a scope-sensitive manner.
func allTemplates(c *chart.Chart, vals chartutil.Values) map[string]renderable {
	templates := make(map[string]renderable)
	recAllTpls(c, templates, vals)
	return templates
}

// recAllTpls recurses through the templates in a chart.
//
// As it recurses, it also sets the values to be appropriate for the template
// scope.
func recAllTpls(c *chart.Chart, templates map[string]renderable, parentVals chartutil.Values) {
	// This should never evaluate to a nil map. That will cause problems when
	// values are appended later.
	cvals := make(chartutil.Values)
	if c.IsRoot() {
		cvals = parentVals
	} else if c.Name() != "" {
		cvals = map[string]interface{}{
			"Values":       make(chartutil.Values),
			"Release":      parentVals["Release"],
			"Chart":        c.Metadata,
			"Files":        chartutil.NewFiles(c.Files),
			"Capabilities": parentVals["Capabilities"],
		}
		// If there is a {{.Values.ThisChart}} in the parent metadata,
		// copy that into the {{.Values}} for this template.
		if vs, err := parentVals.Table("Values." + c.Name()); err == nil {
			cvals["Values"] = vs
		}
	}

	for _, child := range c.Dependencies() {
		recAllTpls(child, templates, cvals)
	}

	newParentID := c.ChartFullPath()
	for _, t := range c.Templates {
		templates[path.Join(newParentID, t.Name)] = renderable{
			tpl:      string(t.Data),
			vals:     cvals,
			basePath: path.Join(newParentID, "templates"),
		}
	}
}
