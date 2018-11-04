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
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint/support"
)

// Values lints a chart's values.yaml file.
func Values(linter *support.Linter) {
	file := "values.yaml"
	vf := filepath.Join(linter.ChartDir, file)
	fileExists := linter.RunLinterRule(support.InfoSev, file, validateValuesFileExistence(vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, file, validateValuesFile(vf))
}

func validateValuesFileExistence(valuesPath string) error {
	_, err := os.Stat(valuesPath)
	if err != nil {
		return errors.Errorf("file does not exist")
	}
	return nil
}

func validateValuesFile(valuesPath string) error {
	_, err := chartutil.ReadValuesFile(valuesPath)
	return errors.Wrap(err, "unable to parse YAML")
}
