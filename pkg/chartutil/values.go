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

package chartutil

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
)

// GlobalKey is the name of the Values key that is used for storing global vars.
const GlobalKey = "global"

// Values represents a collection of chart values.
//
// It should never be stored as an interface{} as conversion to
// map[string]interface{} will then fail (in particular, as value in Values
// or map[string]interface{} structures).
// To avoid this to happen, all function should return map[string]interface{},
// since implicit conversion to Values is allowed anyway.
//
// Values should only be used in one of these cases:
// - when Values methods are used (return type must be map[string]interface{})
// - to store the values for template rendering (as in chart.Values)
type Values map[string]interface{}

// YAML encodes the Values into a YAML string.
func (v Values) YAML() (string, error) {
	b, err := yaml.Marshal(v)
	return string(b), err
}

// Table gets a table (YAML subsection) from a Values object.
//
// The table is returned as a Values.
//
// Compound table names may be specified with dots:
//
//	foo.bar
//
// The above will be evaluated as "The table bar inside the table
// foo".
//
// An ErrNoTable is returned if the table does not exist.
func (v Values) Table(name string) (map[string]interface{}, error) {
	table := v
	var err error

	for _, n := range parsePath(name) {
		if table, err = tableLookup(table, n); err != nil {
			break
		}
	}
	return table, err
}

// Encode writes serialized Values information to the given io.Writer.
func (v Values) Encode(w io.Writer) error {
	out, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func tableLookup(v map[string]interface{}, simple string) (map[string]interface{}, error) {
	v2, ok := v[simple]
	if !ok {
		return v, ErrNoTable{simple}
	}
	if vv, ok := v2.(map[string]interface{}); ok {
		return vv, nil
	}

	return map[string]interface{}{}, ErrNoTable{simple}
}

// ReadValues will parse YAML byte data into a Values.
func ReadValues(data []byte) (vals map[string]interface{}, err error) {
	err = yaml.Unmarshal(data, &vals)
	if len(vals) == 0 {
		vals = map[string]interface{}{}
	}
	return vals, err
}

// ReadValuesFile will parse a YAML file into a map of values.
func ReadValuesFile(filename string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return ReadValues(data)
}

// ReleaseOptions represents the additional release options needed
// for the composition of the final values struct
type ReleaseOptions struct {
	Name      string
	Namespace string
	Revision  int
	IsUpgrade bool
	IsInstall bool
}

// ToRenderValues composes the struct from the data coming from the Releases, Charts and Values files
//
// This takes both ReleaseOptions and Capabilities to merge into the render values.
func ToRenderValues(chrt *chart.Chart, chrtVals map[string]interface{}, options ReleaseOptions, caps *Capabilities) (map[string]interface{}, error) {
	if caps == nil {
		caps = DefaultCapabilities
	}
	top := map[string]interface{}{
		"Chart":        chrt.Metadata,
		"Capabilities": caps,
		"Values":       nil,
		"Release": map[string]interface{}{
			"Name":      options.Name,
			"Namespace": options.Namespace,
			"IsUpgrade": options.IsUpgrade,
			"IsInstall": options.IsInstall,
			"Revision":  options.Revision,
			"Service":   "Helm",
		},
	}

	// if we have an empty map, make sure it is initialized
	if chrtVals == nil {
		top["Values"] = map[string]interface{}{}
	} else {
		vals, err := copystructure.Copy(chrtVals)
		if err != nil {
			return top, err
		}
		top["Values"] = vals.(map[string]interface{})
	}

	return top, nil
}

// istable is a special-purpose function to see if the present thing matches the definition of a YAML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

// PathValue takes a path that traverses a YAML structure and returns the value at the end of that path.
// The path starts at the root of the YAML structure and is comprised of YAML keys separated by periods.
// Given the following YAML data the value at path "chapter.one.title" is "Loomings".
//
//	chapter:
//	  one:
//	    title: "Loomings"
func (v Values) PathValue(path string) (interface{}, error) {
	if path == "" {
		return nil, errors.New("YAML path cannot be empty")
	}
	return v.pathValue(parsePath(path))
}

func (v Values) pathValue(path []string) (interface{}, error) {
	if len(path) == 1 {
		// if exists must be root key not table
		if _, ok := v[path[0]]; ok && !istable(v[path[0]]) {
			return v[path[0]], nil
		}
		return nil, ErrNoValue{path[0]}
	}

	key, path := path[len(path)-1], path[:len(path)-1]
	// get our table for table path
	t, err := v.Table(joinPath(path...))
	if err != nil {
		return nil, ErrNoValue{key}
	}
	// check table for key and ensure value is not a table
	if k, ok := t[key]; ok && !istable(k) {
		return k, nil
	}
	return nil, ErrNoValue{key}
}

func parsePath(key string) []string { return strings.Split(key, ".") }

func joinPath(path ...string) string { return strings.Join(path, ".") }
