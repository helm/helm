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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"gopkg.in/yaml.v2"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/lint/support"
	"k8s.io/helm/pkg/timeconv"
)

// Templates lints the templates in the Linter.
func Templates(linter *support.Linter) {
	templatesPath := filepath.Join(linter.ChartDir, "templates")

	templatesDirExist := linter.RunLinterRule(support.WarningSev, validateTemplatesDir(templatesPath))

	// Templates directory is optional for now
	if !templatesDirExist {
		return
	}

	// Load chart and parse templates, based on tiller/release_server
	chart, err := chartutil.Load(linter.ChartDir)

	chartLoaded := linter.RunLinterRule(support.ErrorSev, validateNoError(err))

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

	renderOk := linter.RunLinterRule(support.ErrorSev, validateNoError(err))

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

		linter.RunLinterRule(support.ErrorSev, validateAllowedExtension(fileName))

		// We only apply the following lint rules to yaml files
		if filepath.Ext(fileName) != ".yaml" {
			continue
		}

		// Check that all the templates have a matching value
		linter.RunLinterRule(support.WarningSev, validateNonMissingValues(fileName, templatesPath, valuesToRender, preExecutedTemplate))

		linter.RunLinterRule(support.WarningSev, validateQuotes(fileName, string(preExecutedTemplate)))

		renderedContent := renderedContentMap[fileName]
		var yamlStruct K8sYamlStruct
		// Even though K8sYamlStruct only defines Metadata namespace, an error in any other
		// key will be raised as well
		err := yaml.Unmarshal([]byte(renderedContent), &yamlStruct)

		validYaml := linter.RunLinterRule(support.ErrorSev, validateYamlContent(fileName, err))

		if !validYaml {
			continue
		}

		linter.RunLinterRule(support.ErrorSev, validateNoNamespace(fileName, yamlStruct))
	}
}

// Validation functions
func validateTemplatesDir(templatesPath string) (lintError support.LintError) {
	if fi, err := os.Stat(templatesPath); err != nil {
		lintError = fmt.Errorf("Directory 'templates/' not found")
	} else if err == nil && !fi.IsDir() {
		lintError = fmt.Errorf("'templates' is not a directory")
	}
	return
}

// Validates that go template tags include the quote pipelined function
// i.e {{ .Foo.bar }} -> {{ .Foo.bar | quote }}
// {{ .Foo.bar }}-{{ .Foo.baz }} -> "{{ .Foo.bar }}-{{ .Foo.baz }}"
func validateQuotes(templateName string, templateContent string) (lintError support.LintError) {
	// {{ .Foo.bar }}
	r, _ := regexp.Compile(`(?m)(:|-)\s+{{[\w|\.|\s|\']+}}\s*$`)
	functions := r.FindAllString(templateContent, -1)

	for _, str := range functions {
		if match, _ := regexp.MatchString("quote", str); !match {
			result := strings.Replace(str, "}}", " | quote }}", -1)
			lintError = fmt.Errorf("templates: \"%s\". Wrap your substitution functions in quotes or use the sprig \"quote\" function: %s -> %s", templateName, str, result)
			return
		}
	}

	// {{ .Foo.bar }}-{{ .Foo.baz }} -> "{{ .Foo.bar }}-{{ .Foo.baz }}"
	r, _ = regexp.Compile(`(?m)({{(\w|\.|\s|\')+}}(\s|-)*)+\s*$`)
	functions = r.FindAllString(templateContent, -1)

	for _, str := range functions {
		result := strings.Replace(str, str, fmt.Sprintf("\"%s\"", str), -1)
		lintError = fmt.Errorf("templates: \"%s\". Wrap your substitution functions in quotes: %s -> %s", templateName, str, result)
		return
	}
	return
}

func validateAllowedExtension(fileName string) (lintError support.LintError) {
	ext := filepath.Ext(fileName)
	validExtensions := []string{".yaml", ".tpl"}

	for _, b := range validExtensions {
		if b == ext {
			return
		}
	}

	lintError = fmt.Errorf("templates: \"%s\" needs to use .yaml or .tpl extensions", fileName)
	return
}

// validateNonMissingValues checks that all the {{}} functions returns a non empty value (<no value> or "")
// and return an error otherwise.
func validateNonMissingValues(fileName string, templatesPath string, chartValues chartutil.Values, templateContent []byte) (lintError support.LintError) {
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

	// 2 - Extract every function and execute them agains the loaded values
	// Supported {{ .Chart.Name }}, {{ .Chart.Name | quote }}
	r, _ := regexp.Compile(`{{[\w|\.|\s|\|\"|\']+}}`)
	functions := r.FindAllString(string(templateContent), -1)

	// Iterate over the {{ FOO }} templates, executing them against the chartValues
	// We do individual templates parsing so we keep the reference for the key (str) that we want it to be interpolated.
	for _, str := range functions {
		newtmpl, err := tmpl.Parse(str)
		if err != nil {
			lintError = fmt.Errorf("templates: %s", err.Error())
			return
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
		lintError = fmt.Errorf("templates: %s: The following functions are not returning any value %v", fileName, emptyValues)
	}
	return
}

func validateNoError(readError error) (lintError support.LintError) {
	if readError != nil {
		lintError = fmt.Errorf("templates: %s", readError.Error())
	}
	return
}

func validateYamlContent(filePath string, err error) (lintError support.LintError) {
	if err != nil {
		lintError = fmt.Errorf("templates: \"%s\". Wrong YAML content", filePath)
	}
	return
}

func validateNoNamespace(filePath string, yamlStruct K8sYamlStruct) (lintError support.LintError) {
	if yamlStruct.Metadata.Namespace != "" {
		lintError = fmt.Errorf("templates: \"%s\". namespace option is currently NOT supported", filePath)
	}
	return
}

// K8sYamlStruct stubs a Kubernetes YAML file.
// Need to access for now to Namespace only
type K8sYamlStruct struct {
	Metadata struct {
		Namespace string
	}
}
