package chartutil

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

// ErrNoTable indicates that a chart does not have a matching table.
var ErrNoTable = errors.New("no table")

// Values represents a collection of chart values.
type Values map[string]interface{}

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
	names := strings.Split(name, ".")
	table := v
	var err error

	for _, n := range names {
		table, err = tableLookup(table, n)
		if err != nil {
			return table, err
		}
	}
	return table, err
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
		return v, ErrNoTable
	}
	vv, ok := v2.(map[string]interface{})
	if !ok {
		return vv, ErrNoTable
	}
	return vv, nil
}

// ReadValues will parse YAML byte data into a Values.
func ReadValues(data []byte) (vals Values, err error) {
	vals = make(map[string]interface{})
	if len(data) > 0 {
		err = yaml.Unmarshal(data, &vals)
	}
	return
}

// ReadValuesFile will parse a YAML file into a map of values.
func ReadValuesFile(filename string) (Values, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return ReadValues(data)
}

// CoalesceValues coalesces all of the values in a chart (and its subcharts).
//
// The overrides map may be used to specifically override configuration values.
func CoalesceValues(chrt *chart.Chart, vals *chart.Config, overrides map[string]interface{}) (Values, error) {
	var cvals Values
	// Parse values if not nil. We merge these at the top level because
	// the passed-in values are in the same namespace as the parent chart.
	if vals != nil {
		evals, err := ReadValues([]byte(vals.Raw))
		if err != nil {
			return cvals, err
		}
		// Override the top-level values. Overrides are NEVER merged deeply.
		// The assumption is that an override is intended to set an explicit
		// and exact value.
		for k, v := range overrides {
			evals[k] = v
		}
		cvals = coalesceValues(chrt, evals)
	} else if len(overrides) > 0 {
		cvals = coalesceValues(chrt, overrides)
	}

	cvals = coalesceDeps(chrt, cvals)

	return cvals, nil
}

// coalesce coalesces the dest values and the chart values, giving priority to the dest values.
//
// This is a helper function for CoalesceValues.
func coalesce(ch *chart.Chart, dest map[string]interface{}) map[string]interface{} {
	dest = coalesceValues(ch, dest)
	coalesceDeps(ch, dest)
	return dest
}

// coalesceDeps coalesces the dependencies of the given chart.
func coalesceDeps(chrt *chart.Chart, dest map[string]interface{}) map[string]interface{} {
	for _, subchart := range chrt.Dependencies {
		if c, ok := dest[subchart.Metadata.Name]; !ok {
			// If dest doesn't already have the key, create it.
			dest[subchart.Metadata.Name] = map[string]interface{}{}
		} else if !istable(c) {
			log.Printf("error: type mismatch on %s: %t", subchart.Metadata.Name, c)
			return dest
		}
		if dv, ok := dest[subchart.Metadata.Name]; ok {
			dest[subchart.Metadata.Name] = coalesce(subchart, dv.(map[string]interface{}))
		}
	}
	return dest
}

// coalesceValues builds up a values map for a particular chart.
//
// Values in v will override the values in the chart.
func coalesceValues(c *chart.Chart, v map[string]interface{}) map[string]interface{} {
	// If there are no values in the chart, we just return the given values
	if c.Values == nil || c.Values.Raw == "" {
		return v
	}

	nv, err := ReadValues([]byte(c.Values.Raw))
	if err != nil {
		// On error, we return just the overridden values.
		// FIXME: We should log this error. It indicates that the YAML data
		// did not parse.
		log.Printf("error reading default values: %s", err)
		return v
	}

	for key, val := range nv {
		if _, ok := v[key]; !ok {
			v[key] = val
		} else if dest, ok := v[key].(Values); ok {
			src, ok := val.(Values)
			if !ok {
				log.Printf("warning: skipped value for %s: Not a table.", key)
				continue
			}
			// coalesce tables
			coalesceTables(dest, src)
		}
	}
	return v
}

// coalesceTables merges a source map into a destination map.
func coalesceTables(dst, src map[string]interface{}) map[string]interface{} {
	for key, val := range src {
		if istable(val) {
			if innerdst, ok := dst[key]; !ok {
				dst[key] = val
			} else if istable(innerdst) {
				coalesceTables(innerdst.(map[string]interface{}), val.(map[string]interface{}))
			} else {
				log.Printf("warning: cannot overwrite table with non table for %s (%v)", key, val)
			}
			continue
		} else if dv, ok := dst[key]; ok && istable(dv) {
			log.Printf("warning: destination for %s is a table. Ignoring non-table value %v", key, val)
			continue
		}
		dst[key] = val
	}
	return dst
}

// istable is a special-purpose function to see if the present thing matches the definition of a YAML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}
