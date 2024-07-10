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

package lint

import (
	"fmt"
    "log"
    "io/ioutil"
	"path/filepath"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/rules"
	"helm.sh/helm/v3/pkg/lint/support"
)
func All(basedir string, values map[string]interface{}, namespace string, _ bool) support.Linter {
	return AllWithKubeVersion(basedir, values, namespace, nil, "")
}
func AllWithKubeVersion(basedir string, values map[string]interface{}, namespace string, kubeVersion *chartutil.KubeVersion, lintIgnoreFile string) support.Linter {
	chartDir, _ := filepath.Abs(basedir)
	var ignorePatterns map[string][]string
	var err error
	if lintIgnoreFile != "" {
		ignorePatterns, err = rules.ParseIgnoreFile(lintIgnoreFile)
		for key, value := range ignorePatterns {
			fmt.Printf("Pattern: %s, Error: %s\n", key, value)
		}
		// Review this to properly handle logging
		log.SetOutput(ioutil.Discard)
		log.Println(err)
	}
	linter := support.Linter{ChartDir: chartDir}
	if rules.IsIgnored(chartDir, ignorePatterns) {
		return linter
	}
	rules.Chartfile(&linter)
	rules.ValuesWithOverrides(&linter, values)
	rules.TemplatesWithKubeVersion(&linter, values, namespace, kubeVersion)
	rules.Dependencies(&linter)
	return linter
}
