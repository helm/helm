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
	"bytes"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

// Engine is an implementation of 'cmd/tiller/environment'.Engine that uses Go templates.
type Engine struct {
	// FuncMap contains the template functions that will be passed to each
	// render call. This may only be modified before the first call to Render.
	FuncMap template.FuncMap
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
	f := FuncMap()
	return &Engine{
		FuncMap: f,
	}
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
func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// Add some extra functionality
	extra := template.FuncMap{
		"toYaml":   chartutil.ToYaml,
		"fromYaml": chartutil.FromYaml,
		"toJson":   chartutil.ToJson,
		"fromJson": chartutil.FromJson,

		// This is a placeholder for the "include" function, which is
		// late-bound to a template. By declaring it here, we preserve the
		// integrity of the linter.
		"include": func(string, interface{}) string { return "not implemented" },
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
}

// alterFuncMap takes the Engine's FuncMap and adds context-specific functions.
//
// The resulting FuncMap is only valid for the passed-in template.
func (e *Engine) alterFuncMap(t *template.Template) template.FuncMap {
	// Clone the func map because we are adding context-specific functions.
	var funcMap template.FuncMap = map[string]interface{}{}
	for k, v := range e.FuncMap {
		funcMap[k] = v
	}

	// Add the 'include' function here so we can close over t.
	funcMap["include"] = func(name string, data interface{}) string {
		buf := bytes.NewBuffer(nil)
		if err := t.ExecuteTemplate(buf, name, data); err != nil {
			buf.WriteString(err.Error())
		}
		return buf.String()
	}

	return funcMap
}

// render takes a map of templates/values and renders them.
func (e *Engine) render(tpls map[string]renderable) (map[string]string, error) {
	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	t := template.New("gotpl")
	if e.Strict {
		t.Option("missingkey=error")
	} else {
		// Not that zero will attempt to add default values for types it knows,
		// but will still emit <no value> for others. We mitigate that later.
		t.Option("missingkey=zero")
	}

	funcMap := e.alterFuncMap(t)

	files := []string{}
	for fname, r := range tpls {
		t = t.New(fname).Funcs(funcMap)
		if _, err := t.Parse(r.tpl); err != nil {
			return map[string]string{}, fmt.Errorf("parse error in %q: %s", fname, err)
		}
		files = append(files, fname)
	}

	rendered := make(map[string]string, len(files))
	var buf bytes.Buffer
	for _, file := range files {
		// At render time, add information about the template that is being rendered.
		vals := tpls[file].vals
		vals["Template"] = map[string]interface{}{"Name": file}
		if err := t.ExecuteTemplate(&buf, file, vals); err != nil {
			return map[string]string{}, fmt.Errorf("render error in %q: %s", file, err)
		}

		// Work around the issue where Go will emit "<no value>" even if Options(missing=zero)
		// is set. Since missing=error will never get here, we do not need to handle
		// the Strict case.
		rendered[file] = strings.Replace(buf.String(), "<no value>", "", -1)
		buf.Reset()
	}

	return rendered, nil
}

// allTemplates returns all templates for a chart and its dependencies.
//
// As it goes, it also prepares the values in a scope-sensitive manner.
func allTemplates(c *chart.Chart, vals chartutil.Values) map[string]renderable {
	templates := map[string]renderable{}
	recAllTpls(c, templates, vals, true, "")
	return templates
}

// recAllTpls recurses through the templates in a chart.
//
// As it recurses, it also sets the values to be appropriate for the template
// scope.
func recAllTpls(c *chart.Chart, templates map[string]renderable, parentVals chartutil.Values, top bool, parentID string) {
	// This should never evaluate to a nil map. That will cause problems when
	// values are appended later.
	cvals := chartutil.Values{}
	if top {
		// If this is the top of the rendering tree, assume that parentVals
		// is already resolved to the authoritative values.
		cvals = parentVals
	} else if c.Metadata != nil && c.Metadata.Name != "" {
		// If there is a {{.Values.ThisChart}} in the parent metadata,
		// copy that into the {{.Values}} for this template.
		newVals := chartutil.Values{}
		if vs, err := parentVals.Table("Values"); err == nil {
			if tmp, err := vs.Table(c.Metadata.Name); err == nil {
				newVals = tmp
			}
		}

		cvals = map[string]interface{}{
			"Values":       newVals,
			"Release":      parentVals["Release"],
			"Chart":        c.Metadata,
			"Files":        chartutil.NewFiles(c.Files),
			"Capabilities": parentVals["Capabilities"],
		}
	}

	newParentID := c.Metadata.Name
	if parentID != "" {
		// We artificially reconstruct the chart path to child templates. This
		// creates a namespaced filename that can be used to track down the source
		// of a particular template declaration.
		newParentID = path.Join(parentID, "charts", newParentID)
	}

	for _, child := range c.Dependencies {
		recAllTpls(child, templates, cvals, false, newParentID)
	}
	for _, t := range c.Templates {
		templates[path.Join(newParentID, t.Name)] = renderable{
			tpl:  string(t.Data),
			vals: cvals,
		}
	}
}
