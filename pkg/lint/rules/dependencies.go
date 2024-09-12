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

package rules // import "helm.sh/helm/v3/pkg/lint/rules"

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/lint/support"
)

// Dependencies runs lints against a chart's dependencies
//
// See https://github.com/helm/helm/issues/7910
func Dependencies(linter *support.Linter) {
	c, err := loader.LoadDir(linter.ChartDir)
	if !linter.RunLinterRule(support.ErrorSev, "", validateChartFormat(err)) {
		return
	}

	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateDependencyInMetadata(c))
	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateDependenciesUnique(c))
	linter.RunLinterRule(support.WarningSev, linter.ChartDir, validateDependencyInChartsDir(c))
}

func validateChartFormat(chartError error) error {
	if chartError != nil {
		return errors.Errorf("unable to load chart\n\t%s", chartError)
	}
	return nil
}

func validateDependencyInChartsDir(c *chart.Chart) (err error) {
	dependencies := map[string]struct{}{}
	missing := []string{}
	for _, dep := range c.Dependencies() {
		dependencies[dep.Metadata.Name] = struct{}{}
	}
	for _, dep := range c.Metadata.Dependencies {
		if _, ok := dependencies[dep.Name]; !ok {
			missing = append(missing, dep.Name)
		}
	}
	if len(missing) > 0 {
		err = fmt.Errorf("chart directory is missing these dependencies: %s", strings.Join(missing, ","))
	}
	return err
}

func validateDependencyInMetadata(c *chart.Chart) (err error) {
	dependencies := map[string]struct{}{}
	missing := []string{}
	for _, dep := range c.Metadata.Dependencies {
		dependencies[dep.Name] = struct{}{}
	}
	for _, dep := range c.Dependencies() {
		if _, ok := dependencies[dep.Metadata.Name]; !ok {
			missing = append(missing, dep.Metadata.Name)
		}
	}
	if len(missing) > 0 {
		err = fmt.Errorf("chart metadata is missing these dependencies: %s", strings.Join(missing, ","))
	}
	return err
}

func validateDependenciesUnique(c *chart.Chart) (err error) {
	dependencies := map[string]*chart.Dependency{}
	shadowing := []string{}

	for _, dep := range c.Metadata.Dependencies {
		key := dep.Name
		if dep.Alias != "" {
			key = dep.Alias
		}
		if dependencies[key] != nil {
			shadowing = append(shadowing, key)
		}
		dependencies[key] = dep
	}
	if len(shadowing) > 0 {
		err = fmt.Errorf("multiple dependencies with name or alias: %s", strings.Join(shadowing, ","))
	}
	return err
}
