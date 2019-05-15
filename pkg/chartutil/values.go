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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"

	"helm.sh/helm/pkg/chart"
)

// ErrNoTable indicates that a chart does not have a matching table.
type ErrNoTable string

func (e ErrNoTable) Error() string { return fmt.Sprintf("%q is not a table", e) }

// ErrNoValue indicates that Values does not contain a key with a value
type ErrNoValue string

func (e ErrNoValue) Error() string { return fmt.Sprintf("%q is not a value", e) }

// GlobalKey is the name of the Values key that is used for storing global vars.
const GlobalKey = "global"

// Values represents a collection of chart values.
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
func (v Values) Table(name string) (Values, error) {
	table := v
	var err error

	for _, n := range parsePath(name) {
		if table, err = tableLookup(table, n); err != nil {
			break
		}
	}
	return table, err
}

// AsMap is a utility function for converting Values to a map[string]interface{}.
//
// It protects against nil map panics.
func (v Values) AsMap() map[string]interface{} {
	if v == nil || len(v) == 0 {
		return map[string]interface{}{}
	}
	return v
}

// Encode writes serialized Values information to the given io.Writer.
func (v Values) Encode(w io.Writer) error {
	//return yaml.NewEncoder(w).Encode(v)
	out, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func tableLookup(v Values, simple string) (Values, error) {
	v2, ok := v[simple]
	if !ok {
		return v, ErrNoTable(simple)
	}
	if vv, ok := v2.(map[string]interface{}); ok {
		return vv, nil
	}

	// This catches a case where a value is of type Values, but doesn't (for some
	// reason) match the map[string]interface{}. This has been observed in the
	// wild, and might be a result of a nil map of type Values.
	if vv, ok := v2.(Values); ok {
		return vv, nil
	}

	return Values{}, ErrNoTable(simple)
}

// ReadValues will parse YAML byte data into a Values.
func ReadValues(data []byte) (vals Values, err error) {
	err = yaml.Unmarshal(data, &vals)
	if len(vals) == 0 {
		vals = Values{}
	}
	return vals, err
}

// ReadValuesFile will parse a YAML file into a map of values.
func ReadValuesFile(filename string) (Values, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return ReadValues(data)
}

// ValidateAgainstSchema checks that values does not violate the structure laid out in schema
func ValidateAgainstSchema(chrt *chart.Chart, values map[string]interface{}) error {
	var sb strings.Builder
	if chrt.Schema != nil {
		err := ValidateAgainstSingleSchema(values, chrt.Schema)
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s:\n", chrt.Name()))
			sb.WriteString(err.Error())
		}
	}

	// For each dependency, recurively call this function with the coalesced values
	for _, subchrt := range chrt.Dependencies() {
		subchrtValues := values[subchrt.Name()].(map[string]interface{})
		if err := ValidateAgainstSchema(subchrt, subchrtValues); err != nil {
			sb.WriteString(err.Error())
		}
	}

	if sb.Len() > 0 {
		return errors.New(sb.String())
	}

	return nil
}

// ValidateAgainstSingleSchema checks that values does not violate the structure laid out in this schema
func ValidateAgainstSingleSchema(values Values, schemaJSON []byte) error {
	valuesData, err := yaml.Marshal(values)
	if err != nil {
		return err
	}
	valuesJSON, err := yaml.YAMLToJSON(valuesData)
	if err != nil {
		return err
	}
	if bytes.Equal(valuesJSON, []byte("null")) {
		valuesJSON = []byte("{}")
	}
	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	valuesLoader := gojsonschema.NewBytesLoader(valuesJSON)

	result, err := gojsonschema.Validate(schemaLoader, valuesLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		var sb strings.Builder
		for _, desc := range result.Errors() {
			sb.WriteString(fmt.Sprintf("- %s\n", desc))
		}
		return errors.New(sb.String())
	}

	return nil
}

// CoalesceValues coalesces all of the values in a chart (and its subcharts).
//
// Values are coalesced together using the following rules:
//
//	- Values in a higher level chart always override values in a lower-level
//		dependency chart
//	- Scalar values and arrays are replaced, maps are merged
//	- A chart has access to all of the variables for it, as well as all of
//		the values destined for its dependencies.
func CoalesceValues(chrt *chart.Chart, vals map[string]interface{}) (Values, error) {
	if vals == nil {
		vals = make(map[string]interface{})
	}
	if _, err := coalesce(chrt, vals); err != nil {
		return vals, err
	}
	return coalesceDeps(chrt, vals)
}

// coalesce coalesces the dest values and the chart values, giving priority to the dest values.
//
// This is a helper function for CoalesceValues.
func coalesce(ch *chart.Chart, dest map[string]interface{}) (map[string]interface{}, error) {
	coalesceValues(ch, dest)
	return coalesceDeps(ch, dest)
}

// coalesceDeps coalesces the dependencies of the given chart.
func coalesceDeps(chrt *chart.Chart, dest map[string]interface{}) (map[string]interface{}, error) {
	for _, subchart := range chrt.Dependencies() {
		if c, ok := dest[subchart.Name()]; !ok {
			// If dest doesn't already have the key, create it.
			dest[subchart.Name()] = make(map[string]interface{})
		} else if !istable(c) {
			return dest, errors.Errorf("type mismatch on %s: %t", subchart.Name(), c)
		}
		if dv, ok := dest[subchart.Name()]; ok {
			dvmap := dv.(map[string]interface{})

			// Get globals out of dest and merge them into dvmap.
			coalesceGlobals(dvmap, dest)

			// Now coalesce the rest of the values.
			var err error
			dest[subchart.Name()], err = coalesce(subchart, dvmap)
			if err != nil {
				return dest, err
			}
		}
	}
	return dest, nil
}

// coalesceGlobals copies the globals out of src and merges them into dest.
//
// For convenience, returns dest.
func coalesceGlobals(dest, src map[string]interface{}) {
	var dg, sg map[string]interface{}

	if destglob, ok := dest[GlobalKey]; !ok {
		dg = make(map[string]interface{})
	} else if dg, ok = destglob.(map[string]interface{}); !ok {
		log.Printf("warning: skipping globals because destination %s is not a table.", GlobalKey)
		return
	}

	if srcglob, ok := src[GlobalKey]; !ok {
		sg = make(map[string]interface{})
	} else if sg, ok = srcglob.(map[string]interface{}); !ok {
		log.Printf("warning: skipping globals because source %s is not a table.", GlobalKey)
		return
	}

	// EXPERIMENTAL: In the past, we have disallowed globals to test tables. This
	// reverses that decision. It may somehow be possible to introduce a loop
	// here, but I haven't found a way. So for the time being, let's allow
	// tables in globals.
	for key, val := range sg {
		if istable(val) {
			vv := copyMap(val.(map[string]interface{}))
			if destv, ok := dg[key]; !ok {
				// Here there is no merge. We're just adding.
				dg[key] = vv
			} else {
				if destvmap, ok := destv.(map[string]interface{}); !ok {
					log.Printf("Conflict: cannot merge map onto non-map for %q. Skipping.", key)
				} else {
					// Basically, we reverse order of coalesce here to merge
					// top-down.
					CoalesceTables(vv, destvmap)
					dg[key] = vv
					continue
				}
			}
		} else if dv, ok := dg[key]; ok && istable(dv) {
			// It's not clear if this condition can actually ever trigger.
			log.Printf("key %s is table. Skipping", key)
			continue
		}
		// TODO: Do we need to do any additional checking on the value?
		dg[key] = val
	}
	dest[GlobalKey] = dg
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(src))
	for k, v := range src {
		m[k] = v
	}
	return m
}

// coalesceValues builds up a values map for a particular chart.
//
// Values in v will override the values in the chart.
func coalesceValues(c *chart.Chart, v map[string]interface{}) {
	for key, val := range c.Values {
		if value, ok := v[key]; ok {
			if value == nil {
				// When the YAML value is null, we remove the value's key.
				// This allows Helm's various sources of values (value files or --set) to
				// remove incompatible keys from any previous chart, file, or set values.
				delete(v, key)
			} else if dest, ok := value.(map[string]interface{}); ok {
				// if v[key] is a table, merge nv's val table into v[key].
				src, ok := val.(map[string]interface{})
				if !ok {
					log.Printf("warning: skipped value for %s: Not a table.", key)
					continue
				}
				// Because v has higher precedence than nv, dest values override src
				// values.
				CoalesceTables(dest, src)
			}
		} else {
			// If the key is not in v, copy it from nv.
			v[key] = val
		}
	}
}

// CoalesceTables merges a source map into a destination map.
//
// dest is considered authoritative.
func CoalesceTables(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil || src == nil {
		return src
	}
	// Because dest has higher precedence than src, dest values override src
	// values.
	for key, val := range src {
		if istable(val) {
			switch innerdst, ok := dst[key]; {
			case !ok:
				dst[key] = val
			case istable(innerdst):
				CoalesceTables(innerdst.(map[string]interface{}), val.(map[string]interface{}))
			default:
				log.Printf("warning: cannot overwrite table with non table for %s (%v)", key, val)
			}
		} else if dv, ok := dst[key]; ok && istable(dv) {
			log.Printf("warning: destination for %s is a table. Ignoring non-table value %v", key, val)
		} else if !ok { // <- ok is still in scope from preceding conditional.
			dst[key] = val
		}
	}
	return dst
}

// ReleaseOptions represents the additional release options needed
// for the composition of the final values struct
type ReleaseOptions struct {
	Name      string
	Namespace string
	IsUpgrade bool
	IsInstall bool
}

// ToRenderValues composes the struct from the data coming from the Releases, Charts and Values files
//
// This takes both ReleaseOptions and Capabilities to merge into the render values.
func ToRenderValues(chrt *chart.Chart, chrtVals map[string]interface{}, options ReleaseOptions, caps *Capabilities) (Values, error) {
	if caps == nil {
		caps = DefaultCapabilities
	}
	top := map[string]interface{}{
		"Chart":        chrt.Metadata,
		"Capabilities": caps,
		"Release": map[string]interface{}{
			"Name":      options.Name,
			"Namespace": options.Namespace,
			"IsUpgrade": options.IsUpgrade,
			"IsInstall": options.IsInstall,
			"Service":   "Helm",
		},
	}

	vals, err := CoalesceValues(chrt, chrtVals)
	if err != nil {
		return top, err
	}

	if err := ValidateAgainstSchema(chrt, vals); err != nil {
		errFmt := "values don't meet the specifications of the schema(s) in the following chart(s):\n%s"
		return top, fmt.Errorf(errFmt, err.Error())
	}

	top["Values"] = vals
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
		return nil, ErrNoValue(path[0])
	}

	key, path := path[len(path)-1], path[:len(path)-1]
	// get our table for table path
	t, err := v.Table(joinPath(path...))
	if err != nil {
		return nil, ErrNoValue(key)
	}
	// check table for key and ensure value is not a table
	if k, ok := t[key]; ok && !istable(k) {
		return k, nil
	}
	return nil, ErrNoValue(key)
}

func parsePath(key string) []string { return strings.Split(key, ".") }

func joinPath(path ...string) string { return strings.Join(path, ".") }
