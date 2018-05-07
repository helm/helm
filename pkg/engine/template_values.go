/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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
	"strconv"
	"strings"

	"k8s.io/helm/pkg/chartutil"
)

type expansionState struct {
	origTop   chartutil.Values
	origVals  chartutil.Values
	engine    *Engine
	prepTpl   *preparedTemplate
	locks     map[string]bool
	valCache  map[string]interface{}
	rendBufs  []*bytes.Buffer
	rendDepth int
}

// ExpandValues will expand all templates found in .Values. The usual .Values structure will be available
// via a new `tval` function instead; eg. {{ .Values.foo.bar }} can be accessed via {{ tval foo.bar }}.
// This permits recursive expansion; eg. a.b='{{a.c}}', a.c='{{a.d}}', a.d='d '=> a.b='d'. If .Values is
// used directly, no recursive expansion will occur. Also, all string values are expanded, so if literal
// {{ }} characters are required, they must be escaped; eg. {{ "{{ }}" }}. Values of any other type (eg.
// numeric) will remain unchanged.
func (engine *Engine) ExpandValues(top chartutil.Values) (chartutil.Values, error) {
	vals, err := top.Table("Values")
	if err != nil {
		return top, nil
	}

	prepTpl, err := engine.renderPrepare()
	if err != nil {
		return nil, fmt.Errorf("Error during templated value expansion: %s", err.Error())
	}

	state := expansionState{
		origTop: top, origVals: vals, engine: engine, prepTpl: prepTpl, locks: map[string]bool{},
		valCache: map[string]interface{}{}, rendBufs: []*bytes.Buffer{}, rendDepth: 0,
	}
	prepTpl.funcMap["tval"] = state.tvalImpl

	var expVals chartutil.Values
	expVals, err = state.expandMapVal(vals, "")
	if err != nil {
		return nil, fmt.Errorf("Error during templated value expansion: %s", err.Error())
	}

	newTop := make(chartutil.Values, len(top))
	for k, v := range top {
		newTop[k] = v
	}
	newTop["Values"] = expVals
	return newTop, nil
}

func (state *expansionState) tvalImpl(path string) (interface{}, error) {
	// Lock state is only checked here. This is the only place in which we can jump to some
	// other subtree of .Values, and if it's unlocked then it's fine to recurse into it. The
	// only time we need to check for another lock is on any further jump (ie. tval call).
	if _, locked := state.locks[path]; locked {
		return "", fmt.Errorf("Cyclic reference to %q", path)
	}
	val, err := state.origVals.PathValue(path)
	if err != nil {
		if val, err = state.origVals.Table(path); err != nil { // PathValue() will not return maps
			return "", fmt.Errorf("Value %q does not exist", path)
		}
	}
	return state.expandVal(val, path)
}

func joinPath(parent string, child string) string {
	if len(parent) == 0 {
		return child
	}
	return parent + "." + child
}

func (state *expansionState) expandMapVal(vals map[string]interface{}, path string) (map[string]interface{}, error) {
	state.locks[path] = true
	defer delete(state.locks, path)

	newVals := make(map[string]interface{}, len(vals))
	for key, val := range vals {
		newVal, err := state.expandVal(val, joinPath(path, key))
		if err != nil {
			return nil, err
		}
		newVals[key] = newVal
	}
	return newVals, nil
}

func (state *expansionState) expandArrayVal(vals []interface{}, path string) ([]interface{}, error) {
	state.locks[path] = true
	defer delete(state.locks, path)

	newVals := make([]interface{}, len(vals))
	for i, val := range vals {
		newVal, err := state.expandVal(val, joinPath(path, strconv.Itoa(i)))
		if err != nil {
			return nil, err
		}
		newVals[i] = newVal
	}
	return newVals, nil
}

func (state *expansionState) expandVal(val interface{}, path string) (interface{}, error) {
	state.locks[path] = true
	defer delete(state.locks, path)

	switch typedVal := val.(type) {
	case chartutil.Values:
		return state.expandMapVal(typedVal, path)
	case map[string]interface{}:
		return state.expandMapVal(typedVal, path)
	case []interface{}:
		return state.expandArrayVal(typedVal, path)
	case string:
		return state.renderVal(path, typedVal)
	default:
		return val, nil
	}
}

func (state *expansionState) renderVal(path string, val string) (interface{}, error) {
	if !strings.Contains(val, "{{") {
		return val, nil
	}

	// We only cache string values that we have actually resolved. It's probably not worthwhile
	// for anything else as either the value is trivial to retrieve (if there are no template
	// expressions) or it is some structure (eg. a list) that will not usually be used directly
	// (and is straightforward to reconstruct from cached values).
	if precalc, found := state.valCache[path]; found {
		return precalc, nil
	}

	tplName := joinPath("__expand", path)
	tpl := state.prepTpl.tpl.New(tplName).Funcs(state.prepTpl.funcMap)
	if _, err := tpl.Parse(val); err != nil {
		return nil, fmt.Errorf("Parse error in value %q: %s", path, err)
	}

	if state.rendDepth == len(state.rendBufs) {
		state.rendBufs = append(state.rendBufs, new(bytes.Buffer))
	}
	state.rendDepth++
	rendered, err := state.engine.renderSingle(tpl, tplName, state.origTop, state.rendBufs[state.rendDepth-1])
	state.rendDepth--
	if err != nil {
		return nil, fmt.Errorf("Error expanding value %q: %s", path, err.Error())
	}
	state.valCache[path] = rendered
	return rendered, nil
}
