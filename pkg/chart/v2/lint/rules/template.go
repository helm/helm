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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/validation"
	apipath "k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/engine"
)

// Templates lints the templates in the Linter.
func Templates(linter *support.Linter, namespace string, values map[string]any, options ...TemplateLinterOption) {
	templateLinter := newTemplateLinter(linter, namespace, values, options...)
	templateLinter.Lint()
}

type TemplateLinterOption func(*templateLinter)

func TemplateLinterKubeVersion(kubeVersion *common.KubeVersion) TemplateLinterOption {
	return func(tl *templateLinter) {
		tl.kubeVersion = kubeVersion
	}
}

func TemplateLinterSkipSchemaValidation(skipSchemaValidation bool) TemplateLinterOption {
	return func(tl *templateLinter) {
		tl.skipSchemaValidation = skipSchemaValidation
	}
}

func newTemplateLinter(linter *support.Linter, namespace string, values map[string]any, options ...TemplateLinterOption) templateLinter {

	result := templateLinter{
		linter:    linter,
		values:    values,
		namespace: namespace,
	}

	for _, o := range options {
		o(&result)
	}

	return result
}

type templateLinter struct {
	linter               *support.Linter
	values               map[string]any
	namespace            string
	kubeVersion          *common.KubeVersion
	skipSchemaValidation bool
}

func (t *templateLinter) Lint() {
	templatesDir := "templates/"
	templatesPath := filepath.Join(t.linter.ChartDir, templatesDir)

	templatesDirExists := t.linter.RunLinterRule(support.WarningSev, templatesDir, templatesDirExists(templatesPath))
	if !templatesDirExists {
		return
	}

	validTemplatesDir := t.linter.RunLinterRule(support.ErrorSev, templatesDir, validateTemplatesDir(templatesPath))
	if !validTemplatesDir {
		return
	}

	// Load chart and parse templates
	chart, err := loader.Load(t.linter.ChartDir)

	chartLoaded := t.linter.RunLinterRule(support.ErrorSev, templatesDir, err)

	if !chartLoaded {
		return
	}

	options := common.ReleaseOptions{
		Name:      "test-release",
		Namespace: t.namespace,
	}

	caps := common.DefaultCapabilities.Copy()
	if t.kubeVersion != nil {
		caps.KubeVersion = *t.kubeVersion
	}

	// lint ignores import-values
	// See https://github.com/helm/helm/issues/9658
	if err := chartutil.ProcessDependencies(chart, t.values); err != nil {
		return
	}

	cvals, err := util.CoalesceValues(chart, t.values)
	if err != nil {
		return
	}

	valuesToRender, err := util.ToRenderValuesWithSchemaValidation(chart, cvals, options, caps, t.skipSchemaValidation)
	if err != nil {
		t.linter.RunLinterRule(support.ErrorSev, templatesDir, err)
		return
	}
	var e engine.Engine
	e.LintMode = true
	renderedContentMap, err := e.Render(chart, valuesToRender)

	renderOk := t.linter.RunLinterRule(support.ErrorSev, templatesDir, err)

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
		fileName := template.Name

		t.linter.RunLinterRule(support.ErrorSev, fileName, validateAllowedExtension(fileName))

		// We only apply the following lint rules to yaml files
		if !isYamlFileExtension(fileName) {
			continue
		}

		// NOTE: disabled for now, Refs https://github.com/helm/helm/issues/1463
		// Check that all the templates have a matching value
		// linter.RunLinterRule(support.WarningSev, fpath, validateNoMissingValues(templatesPath, valuesToRender, preExecutedTemplate))

		// NOTE: disabled for now, Refs https://github.com/helm/helm/issues/1037
		// linter.RunLinterRule(support.WarningSev, fpath, validateQuotes(string(preExecutedTemplate)))

		renderedContent := renderedContentMap[path.Join(chart.Name(), fileName)]
		if strings.TrimSpace(renderedContent) != "" {
			t.linter.RunLinterRule(support.WarningSev, fileName, validateTopIndentLevel(renderedContent))

			decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedContent), 4096)

			// Lint all resources if the file contains multiple documents separated by ---
			for {
				// Even though k8sYamlStruct only defines a few fields, an error in any other
				// key will be raised as well
				var yamlStruct *k8sYamlStruct

				err := decoder.Decode(&yamlStruct)
				if errors.Is(err, io.EOF) {
					break
				}

				//  If YAML linting fails here, it will always fail in the next block as well, so we should return here.
				// fix https://github.com/helm/helm/issues/11391
				if !t.linter.RunLinterRule(support.ErrorSev, fileName, validateYamlContent(err)) {
					return
				}
				if yamlStruct != nil {
					// NOTE: set to warnings to allow users to support out-of-date kubernetes
					// Refs https://github.com/helm/helm/issues/8596
					t.linter.RunLinterRule(support.WarningSev, fileName, validateMetadataName(yamlStruct))
					t.linter.RunLinterRule(support.WarningSev, fileName, validateNoDeprecations(yamlStruct, t.kubeVersion))

					t.linter.RunLinterRule(support.ErrorSev, fileName, validateMatchSelector(yamlStruct, renderedContent))
					t.linter.RunLinterRule(support.ErrorSev, fileName, validateListAnnotations(yamlStruct, renderedContent))
				}
			}
		}
	}
}

// validateTopIndentLevel checks that the content does not start with an indent level > 0.
//
// This error can occur when a template accidentally inserts space. It can cause
// unpredictable errors depending on whether the text is normalized before being passed
// into the YAML parser. So we trap it here.
//
// See https://github.com/helm/helm/issues/8467
func validateTopIndentLevel(content string) error {
	// Read lines until we get to a non-empty one
	scanner := bufio.NewScanner(bytes.NewBufferString(content))
	for scanner.Scan() {
		line := scanner.Text()
		// If line is empty, skip
		if strings.TrimSpace(line) == "" {
			continue
		}
		// If it starts with one or more spaces, this is an error
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return fmt.Errorf("document starts with an illegal indent: %q, which may cause parsing problems", line)
		}
		// Any other condition passes.
		return nil
	}
	return scanner.Err()
}

// Validation functions
func templatesDirExists(templatesPath string) error {
	_, err := os.Stat(templatesPath)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("directory does not exist")
	}
	return nil
}

func validateTemplatesDir(templatesPath string) error {
	fi, err := os.Stat(templatesPath)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func validateAllowedExtension(fileName string) error {
	ext := filepath.Ext(fileName)
	validExtensions := []string{".yaml", ".yml", ".tpl", ".txt"}

	if slices.Contains(validExtensions, ext) {
		return nil
	}

	return fmt.Errorf("file extension '%s' not valid. Valid extensions are .yaml, .yml, .tpl, or .txt", ext)
}

func validateYamlContent(err error) error {
	if err != nil {
		return fmt.Errorf("unable to parse YAML: %w", err)
	}

	return nil
}

// validateMetadataName uses the correct validation function for the object
// Kind, or if not set, defaults to the standard definition of a subdomain in
// DNS (RFC 1123), used by most resources.
func validateMetadataName(obj *k8sYamlStruct) error {
	fn := validateMetadataNameFunc(obj)
	allErrs := field.ErrorList{}
	for _, msg := range fn(obj.Metadata.Name, false) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), obj.Metadata.Name, msg))
	}
	if len(allErrs) > 0 {
		return fmt.Errorf("object name does not conform to Kubernetes naming requirements: %q: %w", obj.Metadata.Name, allErrs.ToAggregate())
	}
	return nil
}

// validateMetadataNameFunc will return a name validation function for the
// object kind, if defined below.
//
// Rules should match those set in the various api validations:
// https://github.com/kubernetes/kubernetes/blob/v1.20.0/pkg/apis/core/validation/validation.go#L205-L274
// https://github.com/kubernetes/kubernetes/blob/v1.20.0/pkg/apis/apps/validation/validation.go#L39
// ...
//
// Implementing here to avoid importing k/k.
//
// If no mapping is defined, returns NameIsDNSSubdomain.  This is used by object
// kinds that don't have special requirements, so is the most likely to work if
// new kinds are added.
func validateMetadataNameFunc(obj *k8sYamlStruct) validation.ValidateNameFunc {
	switch strings.ToLower(obj.Kind) {
	case "pod", "node", "secret", "endpoints", "resourcequota", // core
		"controllerrevision", "daemonset", "deployment", "replicaset", "statefulset", // apps
		"autoscaler",     // autoscaler
		"cronjob", "job", // batch
		"lease",                    // coordination
		"endpointslice",            // discovery
		"networkpolicy", "ingress", // networking
		"podsecuritypolicy",                           // policy
		"priorityclass",                               // scheduling
		"podpreset",                                   // settings
		"storageclass", "volumeattachment", "csinode": // storage
		return validation.NameIsDNSSubdomain
	case "service":
		return validation.NameIsDNS1035Label
	case "namespace":
		return validation.ValidateNamespaceName
	case "serviceaccount":
		return validation.ValidateServiceAccountName
	case "certificatesigningrequest":
		// No validation.
		// https://github.com/kubernetes/kubernetes/blob/v1.20.0/pkg/apis/certificates/validation/validation.go#L137-L140
		return func(_ string, _ bool) []string { return nil }
	case "role", "clusterrole", "rolebinding", "clusterrolebinding":
		// https://github.com/kubernetes/kubernetes/blob/v1.20.0/pkg/apis/rbac/validation/validation.go#L32-L34
		return func(name string, _ bool) []string {
			return apipath.IsValidPathSegmentName(name)
		}
	default:
		return validation.NameIsDNSSubdomain
	}
}

// validateMatchSelector ensures that template specs have a selector declared.
// See https://github.com/helm/helm/issues/1990
func validateMatchSelector(yamlStruct *k8sYamlStruct, manifest string) error {
	switch yamlStruct.Kind {
	case "Deployment", "ReplicaSet", "DaemonSet", "StatefulSet":
		// verify that matchLabels or matchExpressions is present
		if !strings.Contains(manifest, "matchLabels") && !strings.Contains(manifest, "matchExpressions") {
			return fmt.Errorf("a %s must contain matchLabels or matchExpressions, and %q does not", yamlStruct.Kind, yamlStruct.Metadata.Name)
		}
	}
	return nil
}

func validateListAnnotations(yamlStruct *k8sYamlStruct, manifest string) error {
	if yamlStruct.Kind == "List" {
		m := struct {
			Items []struct {
				Metadata struct {
					Annotations map[string]string
				}
			}
		}{}

		if err := yaml.Unmarshal([]byte(manifest), &m); err != nil {
			return validateYamlContent(err)
		}

		for _, i := range m.Items {
			if _, ok := i.Metadata.Annotations["helm.sh/resource-policy"]; ok {
				return errors.New("annotation 'helm.sh/resource-policy' within List objects are ignored")
			}
		}
	}
	return nil
}

func isYamlFileExtension(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".yaml" || ext == ".yml"
}

// k8sYamlStruct stubs a Kubernetes YAML file.
type k8sYamlStruct struct {
	APIVersion string `json:"apiVersion"`
	Kind       string
	Metadata   k8sYamlMetadata
}

type k8sYamlMetadata struct {
	Namespace string
	Name      string
}
