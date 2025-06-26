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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/yaml"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/lint/support"
)

// Crds lints the CRDs in the Linter.
func Crds(linter *support.Linter) {
	fpath := "crds/"
	crdsPath := filepath.Join(linter.ChartDir, fpath)

	crdsDirExist := linter.RunLinterRule(support.WarningSev, fpath, validateCrdsDir(crdsPath))

	// crds directory is optional
	if !crdsDirExist {
		return
	}

	// Load chart and parse CRDs
	chart, err := loader.Load(linter.ChartDir)

	chartLoaded := linter.RunLinterRule(support.ErrorSev, fpath, err)

	if !chartLoaded {
		return
	}

	/* Iterate over all the CRDs to check:
	- It is a YAML file
	- The kind is CustomResourceDefinition
	*/
	for _, crd := range chart.CRDObjects() {
		fileName := crd.Name
		fpath = fileName

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(crd.File.Data), 4096)
		for {
			var yamlStruct *K8sYamlStruct

			err := decoder.Decode(&yamlStruct)
			if err == io.EOF {
				break
			}

			// If YAML linting fails here, it will always fail in the next block as well, so we should return here.
			if !linter.RunLinterRule(support.ErrorSev, fpath, validateYamlContent(err)) {
				return
			}

			linter.RunLinterRule(support.ErrorSev, fpath, validateCrdKind(yamlStruct))
		}
	}
}

// Validation functions
func validateCrdsDir(crdsPath string) error {
	if fi, err := os.Stat(crdsPath); err == nil {
		if !fi.IsDir() {
			return errors.New("not a directory")
		}
	}
	return nil
}

func validateCrdKind(obj *K8sYamlStruct) error {
	if obj.Kind != "CustomResourceDefinition" {
		return fmt.Errorf("object kind is not 'CustomResourceDefinition'")
	}
	return nil
}
