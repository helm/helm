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

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
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
	// Pull out the dependencies of a v1 Chart, since there's no way
	// to tell the serializer to skip a field for just this use case
	savedDependencies := cf.Dependencies
	if cf.APIVersion == chart.APIVersionV1 {
		cf.Dependencies = nil
	}
	out, err := yaml.Marshal(cf)
	if cf.APIVersion == chart.APIVersionV1 {
		cf.Dependencies = savedDependencies
	}
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

	chartYaml := filepath.Join(dirName, ChartfileName)
	if _, err := os.Stat(chartYaml); os.IsNotExist(err) {
		return false, errors.Errorf("no %s exists in directory %q", ChartfileName, dirName)
	}

	chartYamlContent, err := ioutil.ReadFile(chartYaml)
	if err != nil {
		return false, errors.Errorf("cannot read %s in directory %q", ChartfileName, dirName)
	}

	chartContent := new(chart.Metadata)
	if err := yaml.Unmarshal(chartYamlContent, &chartContent); err != nil {
		return false, err
	}
	if chartContent == nil {
		return false, errors.Errorf("chart metadata (%s) missing", ChartfileName)
	}
	if chartContent.Name == "" {
		return false, errors.Errorf("invalid chart (%s): name must not be empty", ChartfileName)
	}

	return true, nil
}
