package engine

import (
	"bytes"
	"fmt"
	"log"
	"text/template"

	"github.com/Masterminds/sprig"
	chartutil "github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/proto/hapi/chart"
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
//
// Values are scoped to their templates. A dependency template will not have
// access to the values set for its parent. If chart "foo" includes chart "bar",
// "bar" will not have access to the values for "foo".
//
// Values are passed through the templates according to scope. If the top layer
// chart includes the chart foo, which includes the chart bar, the values map
// will be examined for a table called "foo". If "foo" is found in vals,
// that section of the values will be passed into the "foo" chart. And if that
// section contains a value named "bar", that value will be passed on to the
// bar chart during render time.
//
// Values are coalesced together using the fillowing rules:
//
//	- Values in a higher level chart always override values in a lower-level
//		dependency chart
//	- Scalar values and arrays are replaced, maps are merged
//	- A chart has access to all of the variables for it, as well as all of
//		the values destined for its dependencies.
func (e *Engine) Render(chrt *chart.Chart, vals *chart.Config, overrides map[string]interface{}) (map[string]string, error) {
	var cvals chartutil.Values

	// Parse values if not nil. We merge these at the top level because
	// the passed-in values are in the same namespace as the parent chart.
	if vals != nil {
		evals, err := chartutil.ReadValues([]byte(vals.Raw))
		if err != nil {
			return map[string]string{}, err
		}
		// Override the top-level values. Overrides are NEVER merged deeply.
		// The assumption is that an override is intended to set an explicit
		// and exact value.
		for k, v := range overrides {
			evals[k] = v
		}
		cvals = coalesceValues(chrt, evals)
	}

	// Render the charts
	tmap := allTemplates(chrt, cvals)
	return e.render(tmap)
}

// renderable is an object that can be rendered.
type renderable struct {
	// tpl is the current template.
	tpl string
	// vals are the values to be supplied to the template.
	vals chartutil.Values
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
	files := []string{}
	for fname, r := range tpls {
		t = t.New(fname).Funcs(e.FuncMap)
		if _, err := t.Parse(r.tpl); err != nil {
			return map[string]string{}, fmt.Errorf("parse error in %q: %s", fname, err)
		}
		files = append(files, fname)
	}

	rendered := make(map[string]string, len(files))
	var buf bytes.Buffer
	for _, file := range files {
		//	log.Printf("Exec %s with %v (%s)", file, tpls[file].vals, tpls[file].tpl)
		if err := t.ExecuteTemplate(&buf, file, tpls[file].vals); err != nil {
			return map[string]string{}, fmt.Errorf("render error in %q: %s", file, err)
		}
		rendered[file] = buf.String()
		buf.Reset()
	}

	return rendered, nil
}

// allTemplates returns all templates for a chart and its dependencies.
//
// As it goes, it also prepares the values in a scope-sensitive manner.
func allTemplates(c *chart.Chart, vals chartutil.Values) map[string]renderable {
	templates := map[string]renderable{}
	recAllTpls(c, templates, vals, true)
	return templates
}

// recAllTpls recurses through the templates in a chart.
//
// As it recurses, it also sets the values to be appropriate for the template
// scope.
func recAllTpls(c *chart.Chart, templates map[string]renderable, parentVals chartutil.Values, top bool) {
	var pvals chartutil.Values
	if top {
		// If this is the top of the rendering tree, assume that parentVals
		// is already resolved to the authoritative values.
		pvals = parentVals
	} else if c.Metadata != nil && c.Metadata.Name != "" {
		// An error indicates that the table doesn't exist. So we leave it as
		// an empty map.
		tmp, err := parentVals.Table(c.Metadata.Name)
		if err == nil {
			pvals = tmp
		}
	}
	cvals := coalesceValues(c, pvals)
	//log.Printf("racAllTpls values: %v", cvals)
	for _, child := range c.Dependencies {
		recAllTpls(child, templates, cvals, false)
	}
	for _, t := range c.Templates {
		templates[t.Name] = renderable{
			tpl:  string(t.Data),
			vals: cvals,
		}
	}
}

// coalesceValues builds up a values map for a particular chart.
//
// Values in v will override the values in the chart.
func coalesceValues(c *chart.Chart, v chartutil.Values) chartutil.Values {
	// If there are no values in the chart, we just return the given values
	if c.Values == nil {
		return v
	}

	nv, err := chartutil.ReadValues([]byte(c.Values.Raw))
	if err != nil {
		// On error, we return just the overridden values.
		// FIXME: We should log this error. It indicates that the TOML data
		// did not parse.
		log.Printf("error reading default values: %s", err)
		return v
	}

	for k, val := range v {
		// NOTE: We could block coalesce on cases where nv does not explicitly
		// declare a value. But that forces the chart author to explicitly
		// set a default for every template param. We want to preserve the
		// possibility of "hidden" parameters.
		if istable(val) {
			if inmap, ok := nv[k]; ok && istable(inmap) {
				coalesceTables(inmap.(map[string]interface{}), val.(map[string]interface{}))
			} else if ok {
				log.Printf("Cannot copy table into non-table value for %s (%v)", k, inmap)
			} else {
				// The parent table does not have a key entry for this item,
				// so we can safely set it. This is necessary for nested charts.
				log.Printf("Copying %s into map %v", k, nv)
				nv[k] = val
			}
		} else {
			nv[k] = val
		}
	}
	return nv
}

// coalesceTables merges a source map into a destination map.
func coalesceTables(dst, src map[string]interface{}) {
	for key, val := range src {
		if istable(val) {
			if innerdst, ok := dst[key]; !ok {
				dst[key] = val
			} else if istable(innerdst) {
				coalesceTables(innerdst.(map[string]interface{}), val.(map[string]interface{}))
			} else {
				log.Printf("Cannot overwrite table with non table for %s (%v)", key, val)
			}
			continue
		} else if dv, ok := dst[key]; ok && istable(dv) {
			log.Printf("Destination for %s is a table. Ignoring non-table value %v", key, val)
			continue
		}
		dst[key] = val
	}
}

// istable is a special-purpose function to see if the present thing matches the definition of a TOML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}
