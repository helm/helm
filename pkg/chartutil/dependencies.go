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

	"github.com/ghodss/yaml"

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

func getAliasDependency(charts []*chart.Chart, aliasChart *chart.Dependency) *chart.Chart {
	var chartFound chart.Chart
	for _, existingChart := range charts {
		if existingChart == nil {
			continue
		}
		if existingChart.Metadata == nil {
			continue
		}
		if existingChart.Metadata.Name != aliasChart.Name {
			continue
		}
		if !version.IsCompatibleRange(aliasChart.Version, existingChart.Metadata.Version) {
			continue
		}
		chartFound = *existingChart
		newMetadata := *existingChart.Metadata
		if aliasChart.Alias != "" {
			newMetadata.Name = aliasChart.Alias
		}
		chartFound.Metadata = &newMetadata
		return &chartFound
	}
	return nil
}

// ProcessDependencyEnabled removes disabled charts from dependencies
func ProcessDependencyEnabled(c *chart.Chart, v map[string]interface{}) error {
	if c.Metadata.Dependencies == nil {
		return nil
	}

	var chartDependencies []*chart.Chart
	// If any dependency is not a part of Chart.yaml
	// then this should be added to chartDependencies.
	// However, if the dependency is already specified in Chart.yaml
	// we should not add it, as it would be anyways processed from Chart.yaml

	for _, existingDependency := range c.Dependencies() {
		var dependencyFound bool
		for _, req := range c.Metadata.Dependencies {
			if existingDependency.Metadata.Name == req.Name && version.IsCompatibleRange(req.Version, existingDependency.Metadata.Version) {
				dependencyFound = true
				break
			}
		}
		if !dependencyFound {
			chartDependencies = append(chartDependencies, existingDependency)
		}
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
	b, _ := yaml.Marshal(v)
	cvals, err := CoalesceValues(c, b)
	if err != nil {
		return err
	}
	// flag dependencies as enabled/disabled
	ProcessDependencyTags(c.Metadata.Dependencies, cvals)
	ProcessDependencyConditions(c.Metadata.Dependencies, cvals)
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

	// recursively call self to process sub dependencies
	for _, t := range cd {
		if err := ProcessDependencyEnabled(t, cvals); err != nil {
			return err
		}
	}
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

// processImportValues merges values from child to parent based on the chart's dependencies' ImportValues field.
func processImportValues(c *chart.Chart) error {
	if c.Metadata.Dependencies == nil {
		return nil
	}
	// combine chart values and empty config to get Values
	cvals, err := CoalesceValues(c, []byte{})
	if err != nil {
		return err
	}
	b := make(map[string]interface{})
	// import values from each dependency if specified in import-values
	for _, r := range c.Metadata.Dependencies {
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
				b = coalesceTables(cvals, pathToMap(parent, vv.AsMap()))
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
				b = coalesceTables(b, vm.AsMap())
			}
		}
		// set our formatted import values
		r.ImportValues = outiv
	}

	// set the new values
	c.Values = coalesceTables(b, cvals)

	return nil
}

// ProcessDependencyImportValues imports specified chart values from child to parent.
func ProcessDependencyImportValues(c *chart.Chart) error {
	for _, d := range c.Dependencies() {
		// recurse
		if err := ProcessDependencyImportValues(d); err != nil {
			return err
		}
	}
	return processImportValues(c)
}
