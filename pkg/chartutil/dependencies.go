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
	"strings"

	"helm.sh/helm/v3/pkg/chart"
)

// ProcessDependencies checks through this chart's dependencies, processing accordingly.
func ProcessDependencies(c *chart.Chart, v Values) error {
	if err := processDependencyEnabled(c, v, ""); err != nil {
		return err
	}
	return processDependencyImportExportValues(c)
}

// processDependencyConditions disables charts based on condition path value in values
func processDependencyConditions(reqs []*chart.Dependency, cvals Values, cpath string) {
	if reqs == nil {
		return
	}
	for _, r := range reqs {
		for _, c := range strings.Split(strings.TrimSpace(r.Condition), ",") {
			if len(c) > 0 {
				// retrieve value
				vv, err := cvals.PathValue(cpath + c)
				if err == nil {
					// if not bool, warn
					if bv, ok := vv.(bool); ok {
						r.Enabled = bv
						break
					} else {
						log.Printf("Warning: Condition path '%s' for chart %s returned non-bool value", c, r.Name)
					}
				} else if _, ok := err.(ErrNoValue); !ok {
					// this is a real error
					log.Printf("Warning: PathValue returned error %v", err)
				}
			}
		}
	}
}

// processDependencyTags disables charts based on tags in values
func processDependencyTags(reqs []*chart.Dependency, cvals Values) {
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

func getAliasDependency(charts []*chart.Chart, dep *chart.Dependency) *chart.Chart {
	for _, c := range charts {
		if c == nil {
			continue
		}
		if c.Name() != dep.Name {
			continue
		}
		if !IsCompatibleRange(dep.Version, c.Metadata.Version) {
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

// processDependencyEnabled removes disabled charts from dependencies
func processDependencyEnabled(c *chart.Chart, v map[string]interface{}, path string) error {
	if c.Metadata.Dependencies == nil {
		return nil
	}

	var chartDependencies []*chart.Chart
	// If any dependency is not a part of Chart.yaml
	// then this should be added to chartDependencies.
	// However, if the dependency is already specified in Chart.yaml
	// we should not add it, as it would be anyways processed from Chart.yaml

Loop:
	for _, existing := range c.Dependencies() {
		for _, req := range c.Metadata.Dependencies {
			if existing.Name() == req.Name && IsCompatibleRange(req.Version, existing.Metadata.Version) {
				continue Loop
			}
		}
		chartDependencies = append(chartDependencies, existing)
	}

	for _, req := range c.Metadata.Dependencies {
		if chartDependency := getAliasDependency(c.Dependencies(), req); chartDependency != nil {
			chartDependencies = append(chartDependencies, chartDependency)
		}
		if req.Alias != "" {
			req.Name = req.Alias
		}
	}
	c.SetDependencies(chartDependencies...)

	// set all to true
	for _, lr := range c.Metadata.Dependencies {
		lr.Enabled = true
	}
	cvals, err := CoalesceValues(c, v)
	if err != nil {
		return err
	}
	// flag dependencies as enabled/disabled
	processDependencyTags(c.Metadata.Dependencies, cvals)
	processDependencyConditions(c.Metadata.Dependencies, cvals, path)
	// make a map of charts to remove
	rm := map[string]struct{}{}
	for _, r := range c.Metadata.Dependencies {
		if !r.Enabled {
			// remove disabled chart
			rm[r.Name] = struct{}{}
		}
	}
	// don't keep disabled charts in new slice
	cd := []*chart.Chart{}
	copy(cd, c.Dependencies()[:0])
	for _, n := range c.Dependencies() {
		if _, ok := rm[n.Metadata.Name]; !ok {
			cd = append(cd, n)
		}
	}
	// don't keep disabled charts in metadata
	cdMetadata := []*chart.Dependency{}
	copy(cdMetadata, c.Metadata.Dependencies[:0])
	for _, n := range c.Metadata.Dependencies {
		if _, ok := rm[n.Name]; !ok {
			cdMetadata = append(cdMetadata, n)
		}
	}

	// recursively call self to process sub dependencies
	for _, t := range cd {
		subpath := path + t.Metadata.Name + "."
		if err := processDependencyEnabled(t, cvals, subpath); err != nil {
			return err
		}
	}
	// set the correct dependencies in metadata
	c.Metadata.Dependencies = nil
	c.Metadata.Dependencies = append(c.Metadata.Dependencies, cdMetadata...)
	c.SetDependencies(cd...)

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

// processImportExportValues merges values between child and parent based on the chart's dependencies' ImportValues
// and ExportValues fields.
func processImportExportValues(c *chart.Chart) error {
	if c.Metadata.Dependencies == nil {
		return nil
	}
	// combine chart values and empty config to get Values
	cvals, err := CoalesceValues(c, nil)
	if err != nil {
		return err
	}
	b := make(map[string]interface{})
	// For each dependency export values from parent or import values from child if export-values or import-values
	// specified.
	for _, r := range c.Metadata.Dependencies {
		b = CoalesceTables(b, getExportedValues(c, r, cvals))
		b = CoalesceTables(b, getImportedValues(r, cvals))
	}

	// set the new values
	c.Values = CoalesceTables(cvals, b)

	return nil
}

// Generates values map which is imported from specified child to parent via import-values.
func getImportedValues(r *chart.Dependency, cvals Values) map[string]interface{} {
	b := make(map[string]interface{})
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
				log.Printf("Warning: ImportValues missing table from chart %s: %v", r.Name, err)
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
	return b
}

// Generates values map which is exported from parent to specified child via export-values.
func getExportedValues(c *chart.Chart, r *chart.Dependency, cvals Values) map[string]interface{} {
	b := make(map[string]interface{})
	var exportValues []interface{}
	for _, rev := range r.ExportValues {
		parent, child, err := parseExportValues(rev, r)
		if err != nil {
			log.Printf("Warning: invalid ExportValues defined in chart %q for its dependency %q: %s", c.Name(), r.Name, err)
			continue
		}

		exportValues = append(exportValues, map[string]string{
			"parent": parent,
			"child":  child,
		})

		var childValMap map[string]interface{}
		// try to get parent table
		vm, err := cvals.Table(parent)
		if err == nil {
			childValMap = pathToMap(child, vm.AsMap())
		} else {
			// still it might be not a table but a simple value
			value, e := cvals.PathValue(parent)
			if e != nil {
				log.Printf("Warning: ExportValues defined in chart %q for its dependency %q can't get the parent path: %v", c.Name(), r.Name, err)
				continue
			}

			childSlice := parsePath(child)
			childLen := len(childSlice)
			if childLen == 1 {
				log.Printf("Warning: in ExportValues defined in chart %q for its dependency %q you are trying to assign a primitive data type (string, int, etc) to the root of your dependent chart values. We will ignore this ExportValues, because this is most likely not what you want. Fix the ExportValues to hide this warning.", c.Name(), r.Name)
				continue
			}

			childValMap = pathToMap(
				joinPath(childSlice[:childLen-1]...),
				map[string]interface{}{
					childSlice[childLen-1]: value,
				},
			)
		}

		// merge new map with values exported to the child into the resulting values map
		b = CoalesceTables(childValMap, b)
	}
	// set formatted export values
	r.ExportValues = exportValues

	return b
}

// Parse and validate export-values.
func parseExportValues(rev interface{}, r *chart.Dependency) (string, string, error) {
	var parent, child string

	switch ev := rev.(type) {
	case map[string]interface{}:
		var ok bool
		parent, ok = ev["parent"].(string)
		if !ok {
			return "", "", fmt.Errorf("parent can't be of null type")
		}
		child, ok = ev["child"].(string)
		if !ok {
			return "", "", fmt.Errorf("child can't be of null type")
		}

		switch parent {
		case "", ".":
			return "", "", fmt.Errorf("parent %q is not allowed", parent)
		}

		switch child {
		case "", ".":
			child = r.Name
		default:
			child = r.Name + "." + child
		}
	case string:
		switch parent = ev; parent {
		case "", ".":
			parent = "exports"
		default:
			parent = "exports." + parent
		}
		child = r.Name
	default:
		return "", "", fmt.Errorf("invalid type of ExportValues")
	}

	return parent, child, nil
}

// processDependencyImportExportValues imports (child to parent) and/or exports (parent to child) values if
// import-values or export-values specified for the dependency.
func processDependencyImportExportValues(c *chart.Chart) error {
	for _, d := range c.Dependencies() {
		// recurse
		if err := processDependencyImportExportValues(d); err != nil {
			return err
		}
	}
	return processImportExportValues(c)
}
