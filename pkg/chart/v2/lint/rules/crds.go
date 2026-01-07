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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

// Crds lints the CRDs in the Linter.
func Crds(linter *support.Linter) {
	fpath := "crds/"
	crdsPath := filepath.Join(linter.ChartDir, fpath)

	// crds directory is optional
	if _, err := os.Stat(crdsPath); errors.Is(err, fs.ErrNotExist) {
		return
	}

	crdsDirValid := linter.RunLinterRule(support.ErrorSev, fpath, validateCrdsDir(crdsPath))
	if !crdsDirValid {
		return
	}

	// Load chart and parse CRDs
	chart, err := loader.Load(linter.ChartDir)

	chartLoaded := linter.RunLinterRule(support.ErrorSev, fpath, err)

	if !chartLoaded {
		return
	}

	/* Iterate over all the CRDs to check:
	1. It is a YAML file and not a template
	2. The API version is apiextensions.k8s.io
	3. The kind is CustomResourceDefinition
	*/
	for _, crd := range chart.CRDObjects() {
		fileName := crd.Name
		fpath = fileName

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(crd.File.Data), 4096)
		for {
			var yamlStruct *k8sYamlStruct

			err := decoder.Decode(&yamlStruct)
			if errors.Is(err, io.EOF) {
				break
			}

			// If YAML parsing fails here, it will always fail in the next block as well, so we should return here.
			// This also confirms the YAML is not a template, since templates can't be decoded into a K8sYamlStruct.
			if !linter.RunLinterRule(support.ErrorSev, fpath, validateYamlContent(err)) {
				return
			}

			if yamlStruct != nil {
				linter.RunLinterRule(support.ErrorSev, fpath, validateCrdAPIVersion(yamlStruct))
				linter.RunLinterRule(support.ErrorSev, fpath, validateCrdKind(yamlStruct))
			}
		}
	}
}

// Validation functions
func validateCrdsDir(crdsPath string) error {
	fi, err := os.Stat(crdsPath)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func validateCrdAPIVersion(obj *k8sYamlStruct) error {
	if !strings.HasPrefix(obj.APIVersion, "apiextensions.k8s.io") {
		return fmt.Errorf("apiVersion is not in 'apiextensions.k8s.io'")
	}
	return nil
}

func validateCrdKind(obj *k8sYamlStruct) error {
	if obj.Kind != "CustomResourceDefinition" {
		return fmt.Errorf("object kind is not 'CustomResourceDefinition'")
	}
	return nil
}
