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
	"log"
	"strings"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/version"
)

// ProcessDependencyConditions disables charts based on condition path value in values
func ProcessDependencyConditions(reqs []*chart.Dependency, cvals Values) {
	if reqs == nil {
		return
	}
	for _, r := range reqs {
		var hasTrue, hasFalse bool
		for _, c := range strings.Split(strings.TrimSpace(r.Condition), ",") {
			if len(c) > 0 {
				// retrieve value
				vv, err := cvals.PathValue(c)
				if err == nil {
					// if not bool, warn
					if bv, ok := vv.(bool); ok {
						if bv {
							hasTrue = true
						} else {
							hasFalse = true
						}
					} else {
						log.Printf("Warning: Condition path '%s' for chart %s returned non-bool value", c, r.Name)
					}
				} else if _, ok := err.(ErrNoValue); !ok {
					// this is a real error
					log.Printf("Warning: PathValue returned error %v", err)
				}
				if vv != nil {
					// got first value, break loop
					break
				}
			}
		}
		if !hasTrue && hasFalse {
			r.Enabled = false
		} else if hasTrue {
			r.Enabled = true

		}
	}
}

// ProcessDependencyTags disables charts based on tags in values
func ProcessDependencyTags(reqs []*chart.Dependency, cvals Values) {
	if reqs == nil {
		return
	}
	vt, err := cvals.Table("tags")
	if err != nil {
		return
	}
	for _, r := range reqs {
		var hasTrue, hasFalse bool
		for _, k := range r.Tags {
			if b, ok := vt[k]; ok {
				// if not bool, warn
				if bv, ok := b.(bool); ok {
					if bv {
						hasTrue = true
					} else {
						hasFalse = true
					}
				} else {
					log.Printf("Warning: Tag '%s' for chart %s returned non-bool value", k, r.Name)
				}
			}
		}
		if !hasTrue && hasFalse {
			r.Enabled = false
		} else if hasTrue || !hasTrue && !hasFalse {
			r.Enabled = true
		}
	}
}

// GetAliasDependency resolves alias dependencies
func GetAliasDependency(charts []*chart.Chart, dep *chart.Dependency) *chart.Chart {
	for _, c := range charts {
		if c == nil {
			continue
		}
		if c.Name() != dep.Name {
			continue
		}
		if !version.IsCompatibleRange(dep.Version, c.Metadata.Version) {
			continue
		}

		out := *c
		md := *c.Metadata
		out.Metadata = &md

		if dep.Alias != "" {
			md.Name = dep.Alias
		}
		return &out
	}
	return nil
}

// pathToMap creates a nested map given a YAML path in dot notation.
func pathToMap(path string, data map[string]interface{}) map[string]interface{} {
	if path == "." {
		return data
	}
	return set(parsePath(path), data)
}

func set(path []string, data map[string]interface{}) map[string]interface{} {
	if len(path) == 0 {
		return nil
	}
	cur := data
	for i := len(path) - 1; i >= 0; i-- {
		cur = map[string]interface{}{path[i]: cur}
	}
	return cur
}

// processImportValues merges values from child to parent based on the chart's dependencies/libraries' ImportValues field.
func processImportValues(c *chart.Chart, isLib bool) error {
	var reqs []*chart.Dependency
	if isLib {
		reqs = c.Metadata.Libraries
	} else {
		reqs = c.Metadata.Dependencies
	}
	if reqs == nil {
		return nil
	}

	// combine chart values and empty config to get Values
	cvals, err := CoalesceValues(c, nil)
	if err != nil {
		return err
	}
	b := make(map[string]interface{})
	// import values from each dependency if specified in import-values
	for _, r := range reqs {
		var outiv []interface{}
		for _, riv := range r.ImportValues {
			switch iv := riv.(type) {
			case map[string]interface{}:
				child := iv["child"].(string)
				parent := iv["parent"].(string)

				outiv = append(outiv, map[string]string{
					"child":  child,
					"parent": parent,
				})

				// get child table
				vv, err := cvals.Table(r.Name + "." + child)
				if err != nil {
					log.Printf("Warning: ImportValues missing table: %v", err)
					continue
				}
				// create value map from child to be merged into parent
				b = CoalesceTables(cvals, pathToMap(parent, vv.AsMap()))
			case string:
				child := "exports." + iv
				outiv = append(outiv, map[string]string{
					"child":  child,
					"parent": ".",
				})
				vm, err := cvals.Table(r.Name + "." + child)
				if err != nil {
					log.Printf("Warning: ImportValues missing table: %v", err)
					continue
				}
				b = CoalesceTables(b, vm.AsMap())
			}
		}
		// set our formatted import values
		r.ImportValues = outiv
	}

	// set the new values
	c.Values = CoalesceTables(b, cvals)

	return nil
}

// ProcessDependencyImportValues imports specified chart values from child to parent.
func ProcessDependencyImportValues(c *chart.Chart, isLib bool) error {
	var reqs []*chart.Chart
	if isLib {
		reqs = c.Libraries()
	} else {
		reqs = c.Dependencies()
	}
	for _, d := range reqs {
		// recurse
		if err := ProcessDependencyImportValues(d, isLib); err != nil {
			return err
		}
	}
	return processImportValues(c, isLib)
}
