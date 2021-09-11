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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	apipath "k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/lint/support"
)

var (
	crdHookSearch     = regexp.MustCompile(`"?helm\.sh/hook"?:\s+crd-install`)
	releaseTimeSearch = regexp.MustCompile(`\.Release\.Time`)
)

// Templates lints the templates in the Linter.
func Templates(linter *support.Linter, values map[string]interface{}, namespace string, strict bool) {
	fpath := "templates/"
	templatesPath := filepath.Join(linter.ChartDir, fpath)

	templatesDirExist := linter.RunLinterRule(support.WarningSev, fpath, validateTemplatesDir(templatesPath))

	// Templates directory is optional for now
	if !templatesDirExist {
		return
	}

	// Load chart and parse templates
	chart, err := loader.Load(linter.ChartDir)

	chartLoaded := linter.RunLinterRule(support.ErrorSev, fpath, err)

	if !chartLoaded {
		return
	}

	options := chartutil.ReleaseOptions{
		Name:      "test-release",
		Namespace: namespace,
	}

	cvals, err := chartutil.CoalesceValues(chart, values)
	if err != nil {
		return
	}
	valuesToRender, err := chartutil.ToRenderValues(chart, cvals, options, nil)
	if err != nil {
		linter.RunLinterRule(support.ErrorSev, fpath, err)
		return
	}
	var e engine.Engine
	e.LintMode = true
	renderedContentMap, err := e.Render(chart, valuesToRender)

	renderOk := linter.RunLinterRule(support.ErrorSev, fpath, err)

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
		fileName, data := template.Name, template.Data
		fpath = fileName

		linter.RunLinterRule(support.ErrorSev, fpath, validateAllowedExtension(fileName))
		// These are v3 specific checks to make sure and warn people if their
		// chart is not compatible with v3
		linter.RunLinterRule(support.WarningSev, fpath, validateNoCRDHooks(data))
		linter.RunLinterRule(support.ErrorSev, fpath, validateNoReleaseTime(data))

		// We only apply the following lint rules to yaml files
		if filepath.Ext(fileName) != ".yaml" || filepath.Ext(fileName) == ".yml" {
			continue
		}

		// NOTE: disabled for now, Refs https://github.com/helm/helm/issues/1463
		// Check that all the templates have a matching value
		// linter.RunLinterRule(support.WarningSev, fpath, validateNoMissingValues(templatesPath, valuesToRender, preExecutedTemplate))

		// NOTE: disabled for now, Refs https://github.com/helm/helm/issues/1037
		// linter.RunLinterRule(support.WarningSev, fpath, validateQuotes(string(preExecutedTemplate)))

		renderedContent := renderedContentMap[path.Join(chart.Name(), fileName)]
		if strings.TrimSpace(renderedContent) != "" {
			linter.RunLinterRule(support.WarningSev, fpath, validateTopIndentLevel(renderedContent))

			decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedContent), 4096)

			// Lint all resources if the file contains multiple documents separated by ---
			for {
				// Even though K8sYamlStruct only defines a few fields, an error in any other
				// key will be raised as well
				var yamlStruct *K8sYamlStruct

				err := decoder.Decode(&yamlStruct)
				if err == io.EOF {
					break
				}

				// If YAML linting fails, we sill progress. So we don't capture the returned state
				// on this linter run.
				linter.RunLinterRule(support.ErrorSev, fpath, validateYamlContent(err))

				if yamlStruct != nil {
					// NOTE: set to warnings to allow users to support out-of-date kubernetes
					// Refs https://github.com/helm/helm/issues/8596
					linter.RunLinterRule(support.WarningSev, fpath, validateMetadataName(yamlStruct))
					linter.RunLinterRule(support.WarningSev, fpath, validateNoDeprecations(yamlStruct))

					linter.RunLinterRule(support.ErrorSev, fpath, validateMatchSelector(yamlStruct, renderedContent))
					linter.RunLinterRule(support.ErrorSev, fpath, validateListAnnotations(yamlStruct, renderedContent))
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
func validateTemplatesDir(templatesPath string) error {
	if fi, err := os.Stat(templatesPath); err != nil {
		return errors.New("directory not found")
	} else if !fi.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func validateAllowedExtension(fileName string) error {
	ext := filepath.Ext(fileName)
	validExtensions := []string{".yaml", ".yml", ".tpl", ".txt"}

	for _, b := range validExtensions {
		if b == ext {
			return nil
		}
	}

	return errors.Errorf("file extension '%s' not valid. Valid extensions are .yaml, .yml, .tpl, or .txt", ext)
}

func validateYamlContent(err error) error {
	return errors.Wrap(err, "unable to parse YAML")
}

// validateMetadataName uses the correct validation function for the object
// Kind, or if not set, defaults to the standard definition of a subdomain in
// DNS (RFC 1123), used by most resources.
func validateMetadataName(obj *K8sYamlStruct) error {
	fn := validateMetadataNameFunc(obj)
	allErrs := field.ErrorList{}
	for _, msg := range fn(obj.Metadata.Name, false) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), obj.Metadata.Name, msg))
	}
	if len(allErrs) > 0 {
		return errors.Wrapf(allErrs.ToAggregate(), "object name does not conform to Kubernetes naming requirements: %q", obj.Metadata.Name)
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
func validateMetadataNameFunc(obj *K8sYamlStruct) validation.ValidateNameFunc {
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
		return func(name string, prefix bool) []string { return nil }
	case "role", "clusterrole", "rolebinding", "clusterrolebinding":
		// https://github.com/kubernetes/kubernetes/blob/v1.20.0/pkg/apis/rbac/validation/validation.go#L32-L34
		return func(name string, prefix bool) []string {
			return apipath.IsValidPathSegmentName(name)
		}
	default:
		return validation.NameIsDNSSubdomain
	}
}

func validateNoCRDHooks(manifest []byte) error {
	if crdHookSearch.Match(manifest) {
		return errors.New("manifest is a crd-install hook. This hook is no longer supported in v3 and all CRDs should also exist the crds/ directory at the top level of the chart")
	}
	return nil
}

func validateNoReleaseTime(manifest []byte) error {
	if releaseTimeSearch.Match(manifest) {
		return errors.New(".Release.Time has been removed in v3, please replace with the `now` function in your templates")
	}
	return nil
}

// validateMatchSelector ensures that template specs have a selector declared.
// See https://github.com/helm/helm/issues/1990
func validateMatchSelector(yamlStruct *K8sYamlStruct, manifest string) error {
	switch yamlStruct.Kind {
	case "Deployment", "ReplicaSet", "DaemonSet", "StatefulSet":
		// verify that matchLabels or matchExpressions is present
		if !(strings.Contains(manifest, "matchLabels") || strings.Contains(manifest, "matchExpressions")) {
			return fmt.Errorf("a %s must contain matchLabels or matchExpressions, and %q does not", yamlStruct.Kind, yamlStruct.Metadata.Name)
		}
	}
	return nil
}
func validateListAnnotations(yamlStruct *K8sYamlStruct, manifest string) error {
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
				return errors.New("Annotation 'helm.sh/resource-policy' within List objects are ignored")
			}
		}
	}
	return nil
}

// K8sYamlStruct stubs a Kubernetes YAML file.
//
// DEPRECATED: In Helm 4, this will be made a private type, as it is for use only within
// the rules package.
type K8sYamlStruct struct {
	APIVersion string `json:"apiVersion"`
	Kind       string
	Metadata   k8sYamlMetadata
}

type k8sYamlMetadata struct {
	Namespace string
	Name      string
}
