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

package rules // import "helm.sh/helm/v3/pkg/lint/rules"

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/asaskevich/govalidator"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
)

// Chartfile runs a set of linter rules related to Chart.yaml file
func Chartfile(linter *support.Linter) {
	chartFileName := "Chart.yaml"
	chartPath := filepath.Join(linter.ChartDir, chartFileName)

	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartYamlNotDirectory(chartPath))

	chartFile, err := chartutil.LoadChartfile(chartPath)
	validChartFile := linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartYamlFormat(err))

	// Guard clause. Following linter rules require a parsable ChartFile
	if !validChartFile {
		return
	}

	// type check for Chart.yaml . ignoring error as any parse
	// errors would already be caught in the above load function
	chartFileForTypeCheck, _ := loadChartFileForTypeCheck(chartPath)

	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartName(chartFile))

	// Chart metadata
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartAPIVersion(chartFile))

	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartVersionType(chartFileForTypeCheck))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartVersion(chartFile))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartAppVersionType(chartFileForTypeCheck))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartMaintainer(chartFile))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartSources(chartFile))
	linter.RunLinterRule(support.InfoSev, chartFileName, validateChartIconPresence(chartFile))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartIconURL(chartFile))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartType(chartFile))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartDependencies(chartFile))
}

func validateChartVersionType(data map[string]interface{}) error {
	return isStringValue(data, "version")
}

func validateChartAppVersionType(data map[string]interface{}) error {
	return isStringValue(data, "appVersion")
}

func isStringValue(data map[string]interface{}, key string) error {
	value, ok := data[key]
	if !ok {
		return nil
	}
	valueType := fmt.Sprintf("%T", value)
	if valueType != "string" {
		return errors.Errorf("%s should be of type string but it's of type %s", key, valueType)
	}
	return nil
}

func validateChartYamlNotDirectory(chartPath string) error {
	fi, err := os.Stat(chartPath)

	if err == nil && fi.IsDir() {
		return errors.New("should be a file, not a directory")
	}
	return nil
}

func validateChartYamlFormat(chartFileError error) error {
	if chartFileError != nil {
		return errors.Errorf("unable to parse YAML\n\t%s", chartFileError.Error())
	}
	return nil
}

func validateChartName(cf *chart.Metadata) error {
	if cf.Name == "" {
		return errors.New("name is required")
	}
	name := filepath.Base(cf.Name)
	if name != cf.Name {
		return fmt.Errorf("chart name %q is invalid", cf.Name)
	}
	return nil
}

func validateChartAPIVersion(cf *chart.Metadata) error {
	if cf.APIVersion == "" {
		return errors.New("apiVersion is required. The value must be either \"v1\" or \"v2\"")
	}

	if cf.APIVersion != chart.APIVersionV1 && cf.APIVersion != chart.APIVersionV2 {
		return fmt.Errorf("apiVersion '%s' is not valid. The value must be either \"v1\" or \"v2\"", cf.APIVersion)
	}

	return nil
}

func validateChartVersion(cf *chart.Metadata) error {
	if cf.Version == "" {
		return errors.New("version is required")
	}

	version, err := semver.NewVersion(cf.Version)

	if err != nil {
		return errors.Errorf("version '%s' is not a valid SemVer", cf.Version)
	}

	c, err := semver.NewConstraint(">0.0.0-0")
	if err != nil {
		return err
	}
	valid, msg := c.Validate(version)

	if !valid && len(msg) > 0 {
		return errors.Errorf("version %v", msg[0])
	}

	return nil
}

func validateChartMaintainer(cf *chart.Metadata) error {
	for _, maintainer := range cf.Maintainers {
		if maintainer.Name == "" {
			return errors.New("each maintainer requires a name")
		} else if maintainer.Email != "" && !govalidator.IsEmail(maintainer.Email) {
			return errors.Errorf("invalid email '%s' for maintainer '%s'", maintainer.Email, maintainer.Name)
		} else if maintainer.URL != "" && !govalidator.IsURL(maintainer.URL) {
			return errors.Errorf("invalid url '%s' for maintainer '%s'", maintainer.URL, maintainer.Name)
		}
	}
	return nil
}

func validateChartSources(cf *chart.Metadata) error {
	for _, source := range cf.Sources {
		if source == "" || !govalidator.IsRequestURL(source) {
			return errors.Errorf("invalid source URL '%s'", source)
		}
	}
	return nil
}

func validateChartIconPresence(cf *chart.Metadata) error {
	if cf.Icon == "" {
		return errors.New("icon is recommended")
	}
	return nil
}

func validateChartIconURL(cf *chart.Metadata) error {
	if cf.Icon != "" && !govalidator.IsRequestURL(cf.Icon) {
		return errors.Errorf("invalid icon URL '%s'", cf.Icon)
	}
	return nil
}

func validateChartDependencies(cf *chart.Metadata) error {
	if len(cf.Dependencies) > 0 && cf.APIVersion != chart.APIVersionV2 {
		return fmt.Errorf("dependencies are not valid in the Chart file with apiVersion '%s'. They are valid in apiVersion '%s'", cf.APIVersion, chart.APIVersionV2)
	}
	return nil
}

func validateChartType(cf *chart.Metadata) error {
	if len(cf.Type) > 0 && cf.APIVersion != chart.APIVersionV2 {
		return fmt.Errorf("chart type is not valid in apiVersion '%s'. It is valid in apiVersion '%s'", cf.APIVersion, chart.APIVersionV2)
	}
	return nil
}

// loadChartFileForTypeCheck loads the Chart.yaml
// in a generic form of a map[string]interface{}, so that the type
// of the values can be checked
func loadChartFileForTypeCheck(filename string) (map[string]interface{}, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	y := make(map[string]interface{})
	err = yaml.Unmarshal(b, &y)
	return y, err
}
