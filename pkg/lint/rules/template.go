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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/lint/support"
	"k8s.io/helm/pkg/timeconv"
)

// Templates lints the templates in the Linter.
func Templates(linter *support.Linter) {
	path := "templates/"
	templatesPath := filepath.Join(linter.ChartDir, path)

	templatesDirExist := linter.RunLinterRule(support.WarningSev, path, validateTemplatesDir(templatesPath))

	// Templates directory is optional for now
	if !templatesDirExist {
		return
	}

	// Load chart and parse templates, based on tiller/release_server
	chart, err := chartutil.Load(linter.ChartDir)

	chartLoaded := linter.RunLinterRule(support.ErrorSev, path, err)

	if !chartLoaded {
		return
	}

	options := chartutil.ReleaseOptions{Name: "testRelease", Time: timeconv.Now(), Namespace: "testNamespace"}
	valuesToRender, err := chartutil.ToRenderValues(chart, chart.Values, options)
	if err != nil {
		// FIXME: This seems to generate a duplicate, but I can't find where the first
		// error is coming from.
		//linter.RunLinterRule(support.ErrorSev, err)
		return
	}
	renderedContentMap, err := engine.New().Render(chart, valuesToRender)

	renderOk := linter.RunLinterRule(support.ErrorSev, path, err)

	if !renderOk {
		return
	}

	/* Iterate over all the templates to check:
	   - It is a .yaml file
		 - All the values in the template file is defined
		 - {{}} include | quote
		 - Generated content is a valid Yaml file
		 - Metadata.Namespace is not set
	*/
	for _, template := range chart.Templates {
		fileName, preExecutedTemplate := template.Name, template.Data
		path = fileName

		linter.RunLinterRule(support.ErrorSev, path, validateAllowedExtension(fileName))

		// We only apply the following lint rules to yaml files
		if filepath.Ext(fileName) != ".yaml" {
			continue
		}

		// Check that all the templates have a matching value
		linter.RunLinterRule(support.WarningSev, path, validateNoMissingValues(templatesPath, valuesToRender, preExecutedTemplate))

		// NOTE, disabled for now, Refs https://github.com/kubernetes/helm/issues/1037
		// linter.RunLinterRule(support.WarningSev, path, validateQuotes(string(preExecutedTemplate)))

		renderedContent := renderedContentMap[filepath.Join(chart.GetMetadata().Name, fileName)]
		var yamlStruct K8sYamlStruct
		// Even though K8sYamlStruct only defines Metadata namespace, an error in any other
		// key will be raised as well
		err := yaml.Unmarshal([]byte(renderedContent), &yamlStruct)

		validYaml := linter.RunLinterRule(support.ErrorSev, path, validateYamlContent(err))

		if !validYaml {
			continue
		}

		linter.RunLinterRule(support.ErrorSev, path, validateNoNamespace(yamlStruct))
	}
}

// Validation functions
func validateTemplatesDir(templatesPath string) error {
	if fi, err := os.Stat(templatesPath); err != nil {
		return errors.New("directory not found")
	} else if err == nil && !fi.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func validateAllowedExtension(fileName string) error {
	ext := filepath.Ext(fileName)
	validExtensions := []string{".yaml", ".tpl", ".txt"}

	for _, b := range validExtensions {
		if b == ext {
			return nil
		}
	}

	return fmt.Errorf("file extension '%s' not valid. Valid extensions are .yaml, .tpl, or .txt", ext)
}

// validateNoMissingValues checks that all the {{}} functions returns a non empty value (<no value> or "")
// and return an error otherwise.
func validateNoMissingValues(templatesPath string, chartValues chartutil.Values, templateContent []byte) error {
	// 1 - Load Main and associated templates
	// Main template that we will parse dynamically
	tmpl := template.New("tpl").Funcs(sprig.TxtFuncMap())
	// If the templatesPath includes any *.tpl files we will parse and import them as associated templates
	associatedTemplates, err := filepath.Glob(filepath.Join(templatesPath, "*.tpl"))

	if len(associatedTemplates) > 0 {
		tmpl, err = tmpl.ParseFiles(associatedTemplates...)
		if err != nil {
			return err
		}
	}

	var buf bytes.Buffer
	var emptyValues []string

	// 2 - Extract every function and execute them against the loaded values
	// Supported {{ .Chart.Name }}, {{ .Chart.Name | quote }}
	r, _ := regexp.Compile(`{{[\w|\.|\s|\|\"|\']+}}`)
	functions := r.FindAllString(string(templateContent), -1)

	// Iterate over the {{ FOO }} templates, executing them against the chartValues
	// We do individual templates parsing so we keep the reference for the key (str) that we want it to be interpolated.
	for _, str := range functions {
		newtmpl, err := tmpl.Parse(str)
		if err != nil {
			return err
		}

		err = newtmpl.ExecuteTemplate(&buf, "tpl", chartValues)

		if err != nil {
			return err
		}

		renderedValue := buf.String()

		if renderedValue == "<no value>" || renderedValue == "" {
			emptyValues = append(emptyValues, str)
		}
		buf.Reset()
	}

	if len(emptyValues) > 0 {
		return fmt.Errorf("these substitution functions are returning no value: %v", emptyValues)
	}
	return nil
}

func validateYamlContent(err error) error {
	if err != nil {
		return fmt.Errorf("unable to parse YAML\n\t%s", err)
	}
	return nil
}

func validateNoNamespace(yamlStruct K8sYamlStruct) error {
	if yamlStruct.Metadata.Namespace != "" {
		return errors.New("namespace option is currently NOT supported")
	}
	return nil
}

// K8sYamlStruct stubs a Kubernetes YAML file.
// Need to access for now to Namespace only
type K8sYamlStruct struct {
	Metadata struct {
		Namespace string
	}
}
