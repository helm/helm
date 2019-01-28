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
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/version"
)

// ProcessLibraries checks through this chart's dependencies, processing accordingly.
func ProcessLibraries(c *chart.Chart, v Values) error {
	if err := processLibraryEnabled(c, v); err != nil {
		return err
	}
	return ProcessDependencyImportValues(c, true)
}

// processLibraryEnabled removes disabled charts from dependencies
func processLibraryEnabled(c *chart.Chart, v map[string]interface{}) error {
	if c.Metadata.Libraries == nil {
		return nil
	}

	var chartLibraries []*chart.Chart
	// If any dependency is not a part of Chart.yaml
	// then this should be added to chartLibraries.
	// However, if the dependency is already specified in Chart.yaml
	// we should not add it, as it would be anyways processed from Chart.yaml

Loop:
	for _, existing := range c.Libraries() {
		for _, req := range c.Metadata.Libraries {
			if existing.Name() == req.Name && version.IsCompatibleRange(req.Version, existing.Metadata.Version) {
				continue Loop
			}
		}
		chartLibraries = append(chartLibraries, existing)
	}

	for _, req := range c.Metadata.Libraries {
		if chartLibrary := GetAliasDependency(c.Libraries(), req); chartLibrary != nil {
			chartLibraries = append(chartLibraries, chartLibrary)
		}
		if req.Alias != "" {
			req.Name = req.Alias
		}
	}
	c.SetLibraries(chartLibraries...)

	// set all to true
	for _, lr := range c.Metadata.Libraries {
		lr.Enabled = true
	}
	cvals, err := CoalesceValues(c, v)
	if err != nil {
		return err
	}
	// flag dependencies as enabled/disabled
	ProcessDependencyTags(c.Metadata.Libraries, cvals)
	ProcessDependencyConditions(c.Metadata.Libraries, cvals)
	// make a map of charts to remove
	rm := map[string]struct{}{}
	for _, r := range c.Metadata.Libraries {
		if !r.Enabled {
			// remove disabled chart
			rm[r.Name] = struct{}{}
		}
	}
	// don't keep disabled charts in new slice
	cd := []*chart.Chart{}
	copy(cd, c.Libraries()[:0])
	for _, n := range c.Libraries() {
		if _, ok := rm[n.Metadata.Name]; !ok {
			cd = append(cd, n)
		}
	}

	// recursively call self to process sub dependencies
	for _, t := range cd {
		if err := processLibraryEnabled(t, cvals); err != nil {
			return err
		}
	}
	c.SetLibraries(cd...)

	return nil
}
