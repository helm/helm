/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint/support"
	"os"
	"path/filepath"
)

// Values lints a chart's values.yaml file.
func Values(linter *support.Linter) {
	vf := filepath.Join(linter.ChartDir, "values.yaml")
	fileExists := linter.RunLinterRule(support.InfoSev, validateValuesFileExistence(linter, vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, validateValuesFile(linter, vf))
}

func validateValuesFileExistence(linter *support.Linter, valuesPath string) (lintError support.LintError) {
	_, err := os.Stat(valuesPath)
	if err != nil {
		lintError = fmt.Errorf("values.yaml file does not exists")
	}
	return
}

func validateValuesFile(linter *support.Linter, valuesPath string) (lintError support.LintError) {
	_, err := chartutil.ReadValuesFile(valuesPath)
	if err != nil {
		lintError = fmt.Errorf("values.yaml is malformed: %s", err.Error())
	}
	return
}
