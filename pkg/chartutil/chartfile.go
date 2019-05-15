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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
)

// LoadChartfile loads a Chart.yaml file into a *chart.Metadata.
func LoadChartfile(filename string) (*chart.Metadata, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	y := new(chart.Metadata)
	err = yaml.Unmarshal(b, y)
	return y, err
}

// SaveChartfile saves the given metadata as a Chart.yaml file at the given path.
//
// 'filename' should be the complete path and filename ('foo/Chart.yaml')
func SaveChartfile(filename string, cf *chart.Metadata) error {
	out, err := yaml.Marshal(cf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, out, 0644)
}

// IsChartDir validate a chart directory.
//
// Checks for a valid Chart.yaml.
func IsChartDir(dirName string) (bool, error) {
	if fi, err := os.Stat(dirName); err != nil {
		return false, err
	} else if !fi.IsDir() {
		return false, errors.Errorf("%q is not a directory", dirName)
	}

	chartYaml := filepath.Join(dirName, "Chart.yaml")
	if _, err := os.Stat(chartYaml); os.IsNotExist(err) {
		return false, errors.Errorf("no Chart.yaml exists in directory %q", dirName)
	}

	chartYamlContent, err := ioutil.ReadFile(chartYaml)
	if err != nil {
		return false, errors.Errorf("cannot read Chart.Yaml in directory %q", dirName)
	}

	chartContent := new(chart.Metadata)
	if err := yaml.Unmarshal(chartYamlContent, &chartContent); err != nil {
		return false, err
	}
	if chartContent == nil {
		return false, errors.New("chart metadata (Chart.yaml) missing")
	}
	if chartContent.Name == "" {
		return false, errors.New("invalid chart (Chart.yaml): name must not be empty")
	}

	return true, nil
}

// IsChartInstallable validates if a chart can be installed
//
// Application chart type is only installable
func IsChartInstallable(chart *chart.Chart) (bool, error) {
	if IsLibraryChart(chart) {
		return false, errors.New("Library charts are not installable")
	}
	validChartType, _ := IsValidChartType(chart)
	if !validChartType {
		return false, errors.New("Invalid chart types are not installable")
	}
	return true, nil
}

// IsValidChartType validates the chart type
//
// Valid types are: application or library
func IsValidChartType(chart *chart.Chart) (bool, error) {
	chartType := chart.Metadata.Type
	if chartType != "" && !strings.EqualFold(chartType, "library") &&
		!strings.EqualFold(chartType, "application") {
		return false, errors.New("Invalid chart type. Valid types are: application or library")
	}
	return true, nil
}

// IsLibraryChart returns true if the chart is a library chart
func IsLibraryChart(c *chart.Chart) bool {
	return strings.EqualFold(c.Metadata.Type, "library")
}

// IsTemplateValid returns true if the template is valid for the chart type
func IsTemplateValid(templateName string, isLibChart bool) bool {
	if isLibChart {
		return strings.HasPrefix(filepath.Base(templateName), "_")
	}
	return true
}
