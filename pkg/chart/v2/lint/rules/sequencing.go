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
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

// Sequencing runs HIP-0025 sequencing lint rules.
func Sequencing(linter *support.Linter) {
	chartPath := filepath.Join(linter.ChartDir, "Chart.yaml")

	chartFile, err := loadChartForSequencing(chartPath)
	if err != nil || chartFile == nil {
		return // Can't lint sequencing without a valid Chart.yaml
	}

	// Validate subchart depends-on references
	linter.RunLinterRule(support.ErrorSev, "Chart.yaml",
		validateSubchartDependsOn(chartFile))

	// Validate no cycles in subchart DAG
	linter.RunLinterRule(support.ErrorSev, "Chart.yaml",
		validateSubchartDAG(chartFile))
}

func loadChartForSequencing(chartPath string) (*chart.Metadata, error) {
	data, err := os.ReadFile(chartPath)
	if err != nil {
		return nil, err
	}
	var md chart.Metadata
	if err := yaml.Unmarshal(data, &md); err != nil {
		return nil, err
	}
	return &md, nil
}

// validateSubchartDependsOn checks that all depends-on references point to known dependencies.
func validateSubchartDependsOn(md *chart.Metadata) error {
	if md == nil {
		return nil
	}

	depNames := make(map[string]bool)
	for _, dep := range md.Dependencies {
		key := dep.Name
		if dep.Alias != "" {
			key = dep.Alias
		}
		depNames[key] = true
	}

	// Check DependsOn field references
	for _, dep := range md.Dependencies {
		key := dep.Name
		if dep.Alias != "" {
			key = dep.Alias
		}
		for _, upstream := range dep.DependsOn {
			if !depNames[upstream] {
				return fmt.Errorf(
					"dependency %q declares depends-on %q, but %q is not a known dependency",
					key, upstream, upstream,
				)
			}
		}
	}

	// Check annotation references
	annotationDeps, err := chartutil.ParseDependsOnSubcharts(md)
	if err != nil {
		return err
	}
	for _, upstream := range annotationDeps {
		if !depNames[upstream] {
			return fmt.Errorf(
				"annotation %s references %q, but %q is not a known dependency",
				chartutil.AnnotationDependsOnSubcharts, upstream, upstream,
			)
		}
	}

	return nil
}

// validateSubchartDAG builds the subchart DAG and checks for cycles.
func validateSubchartDAG(md *chart.Metadata) error {
	if md == nil {
		return nil
	}

	// Only validate if there are sequencing declarations
	hasDependsOn := false
	for _, dep := range md.Dependencies {
		if len(dep.DependsOn) > 0 {
			hasDependsOn = true
			break
		}
	}
	hasAnnotation := false
	if md.Annotations != nil {
		_, hasAnnotation = md.Annotations[chartutil.AnnotationDependsOnSubcharts]
	}
	if !hasDependsOn && !hasAnnotation {
		return nil
	}

	c := &chart.Chart{Metadata: md}
	_, err := chartutil.BuildSubchartDAG(c)
	return err
}
