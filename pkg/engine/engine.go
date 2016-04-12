package engine

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/deis/tiller/pkg/hapi"
)

type Engine struct {
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
func (e *Engine) Render(chart *hapi.Chart, vals *hapi.Values) (map[string]string, error) {
	// Uncomment this once the proto files compile.
	//return render(chart.Chartfile.Name, chart.Templates, vals)
	return map[string]string{}, nil
}

func (e *Engine) render(name string, tpls map[string]string, v interface{}) (map[string]string, error) {
	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	t := template.New(name)
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
