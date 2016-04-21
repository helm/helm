package engine

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"
	chartutil "github.com/deis/tiller/pkg/chart"
	"github.com/deis/tiller/pkg/proto/hapi/chart"
)

// Engine is an implementation of 'cmd/tiller/environment'.Engine that uses Go templates.
type Engine struct {
	// FuncMap contains the template functions that will be passed to each
	// render call. This may only be modified before the first call to Render.
	FuncMap template.FuncMap
}

// New creates a new Go template Engine instance.
//
// The FuncMap is initialized here. You may modify the FuncMap _prior to_ the
// first invocation of Render.
//
// The FuncMap sets all of the Sprig functions except for those that provide
// access to the underlying OS (env, expandenv).
func New() *Engine {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")
	return &Engine{
		FuncMap: f,
	}
}

// Render takes a chart, optional values, and attempts to render the Go templates.
//
// Render can be called repeatedly on the same engine.
//
// This will look in the chart's 'templates' data (e.g. the 'templates/' directory)
// and attempt to render the templates there using the values passed in.
func (e *Engine) Render(chrt *chart.Chart, vals *chart.Config) (map[string]string, error) {
	var cvals chartutil.Values
	if chrt.Values == nil {
		cvals = map[string]interface{}{}
	} else {
		var err error
		cvals, err = chartutil.ReadValues([]byte(chrt.Values.Raw))
		if err != nil {
			return map[string]string{}, err
		}
	}

	// Parse values if not nil
	if vals != nil {
		evals, err := chartutil.ReadValues([]byte(vals.Raw))
		if err != nil {
			return map[string]string{}, err
		}
		// Coalesce chart default values and values
		for k, v := range evals {
			// FIXME: This needs to merge tables. Ideally, this feature should
			// be part of the Values type.
			cvals[k] = v
		}
	}

	// Render the charts
	tmap := allTemplates(chrt)
	return e.render(tmap, cvals)
}

func (e *Engine) render(tpls map[string]string, v interface{}) (map[string]string, error) {
	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	t := template.New("gotpl")
	files := []string{}
	for fname, tpl := range tpls {
		t = t.New(fname).Funcs(e.FuncMap)
		if _, err := t.Parse(tpl); err != nil {
			return map[string]string{}, fmt.Errorf("parse error in %q: %s", fname, err)
		}
		files = append(files, fname)
	}

	rendered := make(map[string]string, len(files))
	var buf bytes.Buffer
	for _, file := range files {
		if err := t.ExecuteTemplate(&buf, file, v); err != nil {
			return map[string]string{}, fmt.Errorf("render error in %q: %s", file, err)
		}
		rendered[file] = buf.String()
		buf.Reset()
	}

	return rendered, nil
}

// allTemplates returns all templates for a chart and its dependencies.
func allTemplates(c *chart.Chart) map[string]string {
	templates := map[string]string{}
	for _, child := range c.Dependencies {
		for _, t := range child.Templates {
			templates[t.Name] = string(t.Data)
		}
	}
	for _, t := range c.Templates {
		templates[t.Name] = string(t.Data)
	}
	return templates
}
