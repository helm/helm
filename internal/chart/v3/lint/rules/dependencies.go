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

package rules

import (
	"fmt"
	"strings"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	"helm.sh/helm/v4/internal/chart/v3/loader"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
)

// Dependencies runs lints against a chart's dependencies
//
// See https://github.com/helm/helm/issues/7910
func Dependencies(linter *support.Linter) {
	DependenciesWithValues(linter, map[string]any{})
}

// DependenciesWithValues runs lints against a chart's dependencies, resolving
// dependency conditions against the chart's values coalesced with valueOverrides.
//
// See https://github.com/helm/helm/issues/7910
func DependenciesWithValues(linter *support.Linter, valueOverrides map[string]any) {
	c, err := loader.LoadDir(linter.ChartDir)
	if !linter.RunLinterRule(support.ErrorSev, "", validateChartFormat(err)) {
		return
	}

	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateDependencyInMetadata(c))
	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateDependenciesUnique(c))
	dependenciesPresent := linter.RunLinterRule(support.WarningSev, linter.ChartDir, validateDependencyInChartsDir(c))
	// Conditions are resolved against the default values of the dependencies
	// too, so only check them when all dependencies are present.
	if dependenciesPresent {
		linter.RunLinterRule(support.WarningSev, linter.ChartDir, validateDependencyConditions(c, valueOverrides))
	}
}

func validateChartFormat(chartError error) error {
	if chartError != nil {
		return fmt.Errorf("unable to load chart\n\t%w", chartError)
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

// validateDependencyConditions checks that the condition of each dependency
// resolves to a value. When none of the values a condition references exists,
// the condition is silently ignored when dependencies are processed, leaving
// the dependency enabled. That is usually an oversight in the chart's default
// values rather than intentional.
//
// See https://github.com/helm/helm/issues/12264
func validateDependencyConditions(c *chart.Chart, valueOverrides map[string]any) error {
	if len(c.Metadata.Dependencies) == 0 {
		return nil
	}
	cvals, err := util.CoalesceValues(c, valueOverrides)
	if err != nil {
		return fmt.Errorf("unable to coalesce chart values: %w", err)
	}

	// The values of an aliased dependency are still keyed by the chart name
	// at this point; they are re-keyed by the alias when dependencies are
	// processed. Map aliases back to chart names so conditions referencing
	// the default values of an aliased dependency resolve correctly.
	aliases := map[string]string{}
	for _, dep := range c.Metadata.Dependencies {
		if dep.Alias != "" {
			aliases[dep.Alias] = dep.Name
		}
	}

	unresolved := []string{}
	for _, dep := range c.Metadata.Dependencies {
		if strings.TrimSpace(dep.Condition) == "" {
			continue
		}
		if !conditionResolves(cvals, aliases, dep.Condition) {
			unresolved = append(unresolved, fmt.Sprintf("%s (condition %q)", dep.Name, dep.Condition))
		}
	}
	if len(unresolved) > 0 {
		return fmt.Errorf("conditions of these dependencies do not resolve to a value and have no effect: %s", strings.Join(unresolved, ", "))
	}
	return nil
}

// conditionResolves reports whether at least one of the comma-separated paths
// of a dependency condition resolves to a value.
func conditionResolves(cvals common.Values, aliases map[string]string, condition string) bool {
	for path := range strings.SplitSeq(strings.TrimSpace(condition), ",") {
		if path == "" {
			continue
		}
		if _, err := cvals.PathValue(path); err == nil {
			return true
		}
		if head, rest, found := strings.Cut(path, "."); found {
			if name, ok := aliases[head]; ok {
				if _, err := cvals.PathValue(name + "." + rest); err == nil {
					return true
				}
			}
		}
	}
	return false
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
