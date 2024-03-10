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

package lint // import "helm.sh/helm/v3/pkg/lint"

import (
	"path/filepath"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/rules"
	"helm.sh/helm/v3/pkg/lint/support"
)

// All runs all of the available linters on the given base directory.
func All(basedir string, values map[string]interface{}, namespace string, _ bool) support.Linter {
	return AllWithKubeVersion(basedir, values, namespace, nil)
}

// AllWithKubeVersion runs all the available linters on the given base directory, allowing to specify the kubernetes version.
func AllWithKubeVersion(basedir string, values map[string]interface{}, namespace string, kubeVersion *chartutil.KubeVersion) support.Linter {
	// Using abs path to get directory context
	chartDir, _ := filepath.Abs(basedir)

	linter := support.Linter{ChartDir: chartDir}
	rules.Chartfile(&linter)
	rules.ValuesWithOverrides(&linter, values)
	rules.TemplatesWithKubeVersion(&linter, values, namespace, kubeVersion)
	rules.Dependencies(&linter)
	return linter
}
