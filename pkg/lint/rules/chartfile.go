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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/asaskevich/govalidator"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint/support"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

// Chartfile runs a set of linter rules related to Chart.yaml file
func Chartfile(linter *support.Linter) {
	chartPath := filepath.Join(linter.ChartDir, "Chart.yaml")

	linter.RunLinterRule(support.ErrorSev, validateChartYamlFileExistence(chartPath))
	linter.RunLinterRule(support.ErrorSev, validateChartYamlNotDirectory(chartPath))

	chartFile, err := chartutil.LoadChartfile(chartPath)
	validChartFile := linter.RunLinterRule(support.ErrorSev, validateChartYamlFormat(err))

	// Guard clause. Following linter rules require a parseable ChartFile
	if !validChartFile {
		return
	}

	linter.RunLinterRule(support.ErrorSev, validateChartName(chartFile))
	linter.RunLinterRule(support.ErrorSev, validateChartNameDirMatch(linter.ChartDir, chartFile))

	// Chart metadata
	linter.RunLinterRule(support.ErrorSev, validateChartVersion(chartFile))
	linter.RunLinterRule(support.ErrorSev, validateChartEngine(chartFile))
	linter.RunLinterRule(support.ErrorSev, validateChartMaintainer(chartFile))
	linter.RunLinterRule(support.ErrorSev, validateChartSources(chartFile))
	linter.RunLinterRule(support.ErrorSev, validateChartHome(chartFile))
}

// Auxiliar validation methods
func validateChartYamlFileExistence(chartPath string) (lintError support.LintError) {
	_, err := os.Stat(chartPath)
	if err != nil {
		lintError = fmt.Errorf("Chart.yaml file does not exist")
	}
	return
}

func validateChartYamlNotDirectory(chartPath string) (lintError support.LintError) {
	fi, err := os.Stat(chartPath)

	if err == nil && fi.IsDir() {
		lintError = fmt.Errorf("Chart.yaml is a directory")
	}
	return
}

func validateChartYamlFormat(chartFileError error) (lintError support.LintError) {
	if chartFileError != nil {
		lintError = fmt.Errorf("Chart.yaml is malformed: %s", chartFileError.Error())
	}
	return
}

func validateChartName(cf *chart.Metadata) (lintError support.LintError) {
	if cf.Name == "" {
		lintError = fmt.Errorf("Chart.yaml: 'name' is required")
	}
	return
}

func validateChartNameDirMatch(chartDir string, cf *chart.Metadata) (lintError support.LintError) {
	if cf.Name != filepath.Base(chartDir) {
		lintError = fmt.Errorf("Chart.yaml: 'name' and directory do not match")
	}
	return
}

func validateChartVersion(cf *chart.Metadata) (lintError support.LintError) {
	if cf.Version == "" {
		lintError = fmt.Errorf("Chart.yaml: 'version' value is required")
		return
	}

	version, err := semver.NewVersion(cf.Version)

	if err != nil {
		lintError = fmt.Errorf("Chart.yaml: version '%s' is not a valid SemVer", cf.Version)
		return
	}

	c, err := semver.NewConstraint("> 0")
	valid, msg := c.Validate(version)

	if !valid && len(msg) > 0 {
		lintError = fmt.Errorf("Chart.yaml: 'version' %v", msg[0])
	}

	return
}

func validateChartEngine(cf *chart.Metadata) (lintError support.LintError) {
	if cf.Engine == "" {
		return
	}

	keys := make([]string, 0, len(chart.Metadata_Engine_value))
	for engine := range chart.Metadata_Engine_value {
		str := strings.ToLower(engine)

		if str == "unknown" {
			continue
		}

		if str == cf.Engine {
			return
		}

		keys = append(keys, str)
	}

	lintError = fmt.Errorf("Chart.yaml: engine '%v' not valid. Valid options are %v", cf.Engine, keys)
	return
}

func validateChartMaintainer(cf *chart.Metadata) (lintError support.LintError) {
	for _, maintainer := range cf.Maintainers {
		if maintainer.Name == "" {
			lintError = fmt.Errorf("Chart.yaml: maintainer requires a name")
		} else if maintainer.Email != "" && !govalidator.IsEmail(maintainer.Email) {
			lintError = fmt.Errorf("Chart.yaml: maintainer invalid email")
		}
	}
	return
}

func validateChartSources(cf *chart.Metadata) (lintError support.LintError) {
	for _, source := range cf.Sources {
		if source == "" || !govalidator.IsRequestURL(source) {
			lintError = fmt.Errorf("Chart.yaml: 'source' invalid URL %s", source)
		}
	}
	return
}

func validateChartHome(cf *chart.Metadata) (lintError support.LintError) {
	if cf.Home != "" && !govalidator.IsRequestURL(cf.Home) {
		lintError = fmt.Errorf("Chart.yaml: 'home' invalid URL %s", cf.Home)
	}
	return
}
