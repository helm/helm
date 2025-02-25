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

package lint // import "helm.sh/helm/v4/pkg/lint"

import (
	"path/filepath"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/lint/rules"
	"helm.sh/helm/v4/pkg/lint/support"
)

type linterOptions struct {
	KubeVersion          *chartutil.KubeVersion
	SkipSchemaValidation bool
}

type LinterOption func(lo *linterOptions)

func WithKubeVersion(kubeVersion *chartutil.KubeVersion) LinterOption {
	return func(lo *linterOptions) {
		lo.KubeVersion = kubeVersion
	}
}

func WithSkipSchemaValidation(skipSchemaValidation bool) LinterOption {
	return func(lo *linterOptions) {
		lo.SkipSchemaValidation = skipSchemaValidation
	}
}

func RunAll(baseDir string, values map[string]interface{}, namespace string, options ...LinterOption) support.Linter {

	chartDir, _ := filepath.Abs(baseDir)

	lo := linterOptions{}
	for _, option := range options {
		option(&lo)
	}

	result := support.Linter{
		ChartDir: chartDir,
	}

	rules.Chartfile(&result)
	rules.ValuesWithOverrides(&result, values)
	rules.TemplatesWithSkipSchemaValidation(&result, values, namespace, lo.KubeVersion, lo.SkipSchemaValidation)
	rules.Dependencies(&result)

	return result
}
