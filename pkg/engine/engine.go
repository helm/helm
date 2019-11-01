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
	"log"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

// Engine is an implementation of 'cmd/tiller/environment'.Engine that uses Go templates.
type Engine struct {
	// If strict is enabled, template rendering will fail if a template references
	// a value that was not passed in.
	Strict bool
	// In LintMode, some 'required' template values may be missing, so don't fail
	LintMode bool
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
func (e Engine) Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	// parse values templates and update values and dependencies
	if err := e.updateRenderValues(chrt, values); err != nil {
		return map[string]string{}, err
	}
	// parse templates with the updated values
	tmap := allTemplates(chrt, values)
	return e.render(tmap)
}

// Render takes a chart, optional values, and value overrides, and attempts to
// render the Go templates using the default options.
func Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	return new(Engine).Render(chrt, values)
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

const warnStartDelim = "HELM_ERR_START"
const warnEndDelim = "HELM_ERR_END"

var warnRegex = regexp.MustCompile(warnStartDelim + `(.*)` + warnEndDelim)

func warnWrap(warn string) string {
	return warnStartDelim + warn + warnEndDelim
}

// initFunMap creates the Engine's FuncMap and adds context-specific functions.
func (e Engine) initFunMap(t *template.Template, referenceTpls map[string]renderable) {
	funcMap := funcMap()

	// Add the 'include' function here so we can close over t.
	funcMap["include"] = func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		err := t.ExecuteTemplate(&buf, name, data)
		return buf.String(), err
	}

	// Add the 'tpl' function here
	funcMap["tpl"] = func(tpl string, vals chartutil.Values) (string, error) {
		basePath, err := vals.PathValue("Template.BasePath")
		if err != nil {
			return "", errors.Wrapf(err, "cannot retrieve Template.Basepath from values inside tpl function: %s", tpl)
		}

		templateName, err := vals.PathValue("Template.Name")
		if err != nil {
			return "", errors.Wrapf(err, "cannot retrieve Template.Name from values inside tpl function: %s", tpl)
		}

		templates := map[string]renderable{
			templateName.(string): {
				tpl:      tpl,
				vals:     vals,
				basePath: basePath.(string),
			},
		}

		result, err := e.renderWithReferences(templates, referenceTpls)
		if err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}
		return result[templateName.(string)], nil
	}

	// Add the `required` function here so we can use lintMode
	funcMap["required"] = func(warn string, val interface{}) (interface{}, error) {
		if val == nil {
			if e.LintMode {
				// Don't fail on missing required values when linting
				log.Printf("[INFO] Missing required value: %s", warn)
				return "", nil
			}
			return val, errors.Errorf(warnWrap(warn))
		} else if _, ok := val.(string); ok {
			if val == "" {
				if e.LintMode {
					// Don't fail on missing required values when linting
					log.Printf("[INFO] Missing required value: %s", warn)
					return "", nil
				}
				return val, errors.Errorf(warnWrap(warn))
			}
		}
		return val, nil
	}

	t.Funcs(funcMap)
}

// render takes a map of templates/values and renders them.
func (e Engine) render(tpls map[string]renderable) (map[string]string, error) {
	return e.renderWithReferences(tpls, tpls)
}

// renderWithReferences takes a map of templates/values to render, and a map of
// templates which can be referenced within them.
func (e Engine) renderWithReferences(tpls, referenceTpls map[string]renderable) (rendered map[string]string, err error) {
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

	e.initFunMap(t, referenceTpls)

	// We want to parse the templates in a predictable order. The order favors
	// higher-level (in file system) templates over deeply nested templates.
	keys := sortTemplates(tpls)

	for _, filename := range keys {
		r := tpls[filename]
		if _, err := t.New(filename).Parse(r.tpl); err != nil {
			return map[string]string{}, cleanupParseError(filename, err)
		}
	}

	// Adding the reference templates to the template context
	// so they can be referenced in the tpl function
	for filename, r := range referenceTpls {
		if t.Lookup(filename) == nil {
			if _, err := t.New(filename).Parse(r.tpl); err != nil {
				return map[string]string{}, cleanupParseError(filename, err)
			}
		}
	}

	rendered = make(map[string]string, len(keys))
	for _, filename := range keys {
		// Don't render partials. We don't care out the direct output of partials.
		// They are only included from other templates.
		if strings.HasPrefix(path.Base(filename), "_") {
			continue
		}
		// At render time, add information about the template that is being rendered.
		vals := tpls[filename].vals
		vals["Template"] = map[string]interface{}{"Name": filename, "BasePath": tpls[filename].basePath}
		var buf strings.Builder
		if err := t.ExecuteTemplate(&buf, filename, vals); err != nil {
			return map[string]string{}, cleanupExecError(filename, err)
		}

		// Work around the issue where Go will emit "<no value>" even if Options(missing=zero)
		// is set. Since missing=error will never get here, we do not need to handle
		// the Strict case.
		rendered[filename] = strings.ReplaceAll(buf.String(), "<no value>", "")
	}

	return rendered, nil
}

func cleanupParseError(filename string, err error) error {
	tokens := strings.Split(err.Error(), ": ")
	if len(tokens) == 1 {
		// This might happen if a non-templating error occurs
		return fmt.Errorf("parse error in (%s): %s", filename, err)
	}
	// The first token is "template"
	// The second token is either "filename:lineno" or "filename:lineNo:columnNo"
	location := tokens[1]
	// The remaining tokens make up a stacktrace-like chain, ending with the relevant error
	errMsg := tokens[len(tokens)-1]
	return fmt.Errorf("parse error at (%s): %s", string(location), errMsg)
}

func cleanupExecError(filename string, err error) error {
	if _, isExecError := err.(template.ExecError); !isExecError {
		return err
	}

	tokens := strings.SplitN(err.Error(), ": ", 3)
	if len(tokens) != 3 {
		// This might happen if a non-templating error occurs
		return fmt.Errorf("execution error in (%s): %s", filename, err)
	}

	// The first token is "template"
	// The second token is either "filename:lineno" or "filename:lineNo:columnNo"
	location := tokens[1]

	parts := warnRegex.FindStringSubmatch(tokens[2])
	if len(parts) >= 2 {
		return fmt.Errorf("execution error at (%s): %s", string(location), parts[1])
	}

	return err
}

// updateRenderValues update render values with chart values and values templates.
func (e Engine) updateRenderValues(c *chart.Chart, vals chartutil.Values) error {
	var sb strings.Builder
	// parse values templates and update values and dependencies
	if err := e.recUpdateRenderValues(c, vals, nil, &sb); err != nil {
		return err
	}
	// Check for values validation errors
	if sb.Len() > 0 {
		errFmt := "values don't meet the specifications of the schema(s) in the following chart(s):\n%s"
		return fmt.Errorf(errFmt, sb.String())
	}
	// import values from dependenvies
	if err := chartutil.ProcessDependencyImportValues(c, vals["Values"].(map[string]interface{})); err != nil {
		return err
	}

	return nil
}

func (e Engine) recUpdateRenderValues(c *chart.Chart, vals chartutil.Values, tags map[string]interface{}, sb *strings.Builder) error {
	next := map[string]interface{}{
		"Chart":        c.Metadata,
		"Files":        newFiles(c.Files),
		"Release":      vals["Release"],
		"Capabilities": vals["Capabilities"],
		"Values":       nil,
	}

	// If there is a {{.Values.ThisChart}} in the parent metadata,
	// copy that into the {{.Values}} for this template.
	var nvals map[string]interface{}
	var err error
	if c.IsRoot() {
		nvals, err = chartutil.CoalesceRoot(c, vals["Values"].(map[string]interface{}))
	} else {
		nvals, err = chartutil.CoalesceDep(c, vals["Values"].(map[string]interface{}))
	}
	if err != nil {
		return err
	}
	next["Values"] = nvals
	// Get validations errors of chart values, before applying values template
	if c.Schema != nil {
		err = chartutil.ValidateAgainstSingleSchema(nvals, c.Schema)
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s:\n", c.Name()))
			sb.WriteString(err.Error())
		}
	}
	// Get all values templates of the chart
	templates := make(map[string]renderable)
	newParentID := c.ChartFullPath()
	for _, t := range c.ValuesTemplates {
		if !isTemplateValid(c, t.Name) {
			continue
		}
		templates[path.Join(newParentID, t.Name)] = renderable{
			tpl:      string(t.Data),
			vals:     next,
			basePath: path.Join(newParentID, "values"),
		}
	}
	// Render all values templates
	rendered, err := e.render(templates)
	if err != nil {
		return err
	}
	// Parse and apply all values templates
	if len(rendered) > 0 {
		for _, filename := range sortValuesTemplates(rendered) {
			src := make(map[string]interface{})
			if err := yaml.Unmarshal([]byte(rendered[filename]), &src); err != nil {
				return errors.Wrap(err, fmt.Sprintf("cannot load %s", filename))
			}
			chartutil.CoalesceTablesUpdate(nvals, src)
		}
	}
	// Get tags of the root
	if c.IsRoot() {
		tags = chartutil.GetTags(nvals)
	}
	// Remove all disabled dependencies
	err = chartutil.ProcessDependencyEnabled(c, nvals, tags)
	if err != nil {
		return err
	}
	// Recursive upudate on enabled dependencies
	for _, child := range c.Dependencies() {
		err = e.recUpdateRenderValues(child, next, tags, sb)
		if err != nil {
			return err
		}
	}
	return nil
}

// sortValuesTemplates sorts the rendered yaml values files from lowest to highest priority
func sortValuesTemplates(tpls map[string]string) []string {
	keys := make(sort.StringSlice, len(tpls))
	i := 0
	for key := range tpls {
		keys[i] = key
		i++
	}
	sort.Sort(keys)
	// sort.Sort(sort.Reverse(keys))
	return keys
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
func recAllTpls(c *chart.Chart, templates map[string]renderable, vals chartutil.Values) {
	next := map[string]interface{}{
		"Chart":        c.Metadata,
		"Files":        newFiles(c.Files),
		"Release":      vals["Release"],
		"Capabilities": vals["Capabilities"],
		"Values":       make(chartutil.Values),
	}

	// If there is a {{.Values.ThisChart}} in the parent metadata,
	// copy that into the {{.Values}} for this template.
	if c.IsRoot() {
		next["Values"] = vals["Values"]
	} else if vs, err := vals.Table("Values." + c.Name()); err == nil {
		next["Values"] = vs
	}

	for _, child := range c.Dependencies() {
		recAllTpls(child, templates, next)
	}

	newParentID := c.ChartFullPath()
	for _, t := range c.Templates {
		if !isTemplateValid(c, t.Name) {
			continue
		}
		templates[path.Join(newParentID, t.Name)] = renderable{
			tpl:      string(t.Data),
			vals:     next,
			basePath: path.Join(newParentID, "templates"),
		}
	}
}

// isTemplateValid returns true if the template is valid for the chart type
func isTemplateValid(ch *chart.Chart, templateName string) bool {
	if isLibraryChart(ch) {
		return strings.HasPrefix(filepath.Base(templateName), "_")
	}
	return true
}

// isLibraryChart returns true if the chart is a library chart
func isLibraryChart(c *chart.Chart) bool {
	return strings.EqualFold(c.Metadata.Type, "library")
}
