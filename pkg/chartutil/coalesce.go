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
	"fmt"
	"log"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chart"
)

func concatPrefix(a, b string) string {
	if a == "" {
		return b
	}
	return fmt.Sprintf("%s.%s", a, b)
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
	v, err := copystructure.Copy(vals)
	if err != nil {
		return vals, err
	}

	valsCopy := v.(map[string]interface{})
	// if we have an empty map, make sure it is initialized
	if valsCopy == nil {
		valsCopy = make(map[string]interface{})
	}
	return coalesce(log.Printf, chrt, valsCopy, "")
}

type printFn func(format string, v ...interface{})

// coalesce coalesces the dest values and the chart values, giving priority to the dest values.
//
// This is a helper function for CoalesceValues.
func coalesce(printf printFn, ch *chart.Chart, dest map[string]interface{}, prefix string) (map[string]interface{}, error) {
	coalesceValues(printf, ch, dest, prefix)
	return coalesceDeps(printf, ch, dest, prefix)
}

// coalesceDeps coalesces the dependencies of the given chart.
func coalesceDeps(printf printFn, chrt *chart.Chart, dest map[string]interface{}, prefix string) (map[string]interface{}, error) {
	for _, subchart := range chrt.Dependencies() {
		if c, ok := dest[subchart.Name()]; !ok {
			// If dest doesn't already have the key, create it.
			dest[subchart.Name()] = make(map[string]interface{})
		} else if !istable(c) {
			return dest, errors.Errorf("type mismatch on %s: %t", subchart.Name(), c)
		}
		if dv, ok := dest[subchart.Name()]; ok {
			dvmap := dv.(map[string]interface{})
			subPrefix := concatPrefix(prefix, chrt.Metadata.Name)

			// Get globals out of dest and merge them into dvmap.
			coalesceGlobals(printf, dvmap, dest, subPrefix)

			// Now coalesce the rest of the values.
			var err error
			dest[subchart.Name()], err = coalesce(printf, subchart, dvmap, subPrefix)
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
func coalesceGlobals(printf printFn, dest, src map[string]interface{}, prefix string) {
	var dg, sg map[string]interface{}

	if destglob, ok := dest[GlobalKey]; !ok {
		dg = make(map[string]interface{})
	} else if dg, ok = destglob.(map[string]interface{}); !ok {
		printf("warning: skipping globals because destination %s is not a table.", GlobalKey)
		return
	}

	if srcglob, ok := src[GlobalKey]; !ok {
		sg = make(map[string]interface{})
	} else if sg, ok = srcglob.(map[string]interface{}); !ok {
		printf("warning: skipping globals because source %s is not a table.", GlobalKey)
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
					printf("Conflict: cannot merge map onto non-map for %q. Skipping.", key)
				} else {
					// Basically, we reverse order of coalesce here to merge
					// top-down.
					subPrefix := concatPrefix(prefix, key)
					coalesceTablesFullKey(printf, vv, destvmap, subPrefix)
					dg[key] = vv
				}
			}
		} else if dv, ok := dg[key]; ok && istable(dv) {
			// It's not clear if this condition can actually ever trigger.
			printf("key %s is table. Skipping", key)
		} else {
			// TODO: Do we need to do any additional checking on the value?
			dg[key] = val
		}
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
func coalesceValues(printf printFn, c *chart.Chart, v map[string]interface{}, prefix string) {
	subPrefix := concatPrefix(prefix, c.Metadata.Name)
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
					// If the original value is nil, there is nothing to coalesce, so we don't print
					// the warning
					if val != nil {
						printf("warning: skipped value for %s.%s: Not a table.", subPrefix, key)
					}
				} else {
					// Because v has higher precedence than nv, dest values override src
					// values.
					coalesceTablesFullKey(printf, dest, src, concatPrefix(subPrefix, key))
				}
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
	return coalesceTablesFullKey(log.Printf, dst, src, "")
}

// coalesceTablesFullKey merges a source map into a destination map.
//
// dest is considered authoritative.
func coalesceTablesFullKey(printf printFn, dst, src map[string]interface{}, prefix string) map[string]interface{} {
	// When --reuse-values is set but there are no modifications yet, return new values
	if src == nil {
		return dst
	}
	if dst == nil {
		return src
	}
	// Because dest has higher precedence than src, dest values override src
	// values.
	for key, val := range src {
		fullkey := concatPrefix(prefix, key)
		if dv, ok := dst[key]; ok && dv == nil {
			delete(dst, key)
		} else if !ok {
			dst[key] = val
		} else if istable(val) {
			if istable(dv) {
				coalesceTablesFullKey(printf, dv.(map[string]interface{}), val.(map[string]interface{}), fullkey)
			} else {
				printf("warning: cannot overwrite table with non table for %s (%v)", fullkey, val)
			}
		} else if istable(dv) && val != nil {
			printf("warning: destination for %s is a table. Ignoring non-table value (%v)", fullkey, val)
		}
	}
	return dst
}
