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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
)

const (
	badChartDir        = "testdata/badchartfile"
	anotherBadChartDir = "testdata/anotherbadchartfile"
)

var (
	badChartFilePath         = filepath.Join(badChartDir, "Chart.yaml")
	nonExistingChartFilePath = filepath.Join(os.TempDir(), "Chart.yaml")
)

var badChart, _ = chartutil.LoadChartfile(badChartFilePath)

// Validation functions Test
func TestValidateChartYamlNotDirectory(t *testing.T) {
	_ = os.Mkdir(nonExistingChartFilePath, os.ModePerm)
	defer os.Remove(nonExistingChartFilePath)

	err := validateChartYamlNotDirectory(nonExistingChartFilePath)
	if err == nil {
		t.Errorf("validateChartYamlNotDirectory to return a linter error, got no error")
	}
}

func TestValidateChartYamlFormat(t *testing.T) {
	err := validateChartYamlFormat(errors.New("Read error"))
	if err == nil {
		t.Errorf("validateChartYamlFormat to return a linter error, got no error")
	}

	err = validateChartYamlFormat(nil)
	if err != nil {
		t.Errorf("validateChartYamlFormat to return no error, got a linter error")
	}
}

func TestValidateChartName(t *testing.T) {
	err := validateChartName(badChart)
	if err == nil {
		t.Errorf("validateChartName to return a linter error, got no error")
	}
}

func TestValidateChartVersion(t *testing.T) {
	var failTest = []struct {
		Version  string
		ErrorMsg string
	}{
		{"", "version is required"},
		{"1.2.3.4", "version '1.2.3.4' is not a valid SemVer"},
		{"waps", "'waps' is not a valid SemVer"},
		{"-3", "'-3' is not a valid SemVer"},
	}

	var successTest = []string{"0.0.1", "0.0.1+build", "0.0.1-beta"}

	for _, test := range failTest {
		badChart.Version = test.Version
		err := validateChartVersion(badChart)
		if err == nil || !strings.Contains(err.Error(), test.ErrorMsg) {
			t.Errorf("validateChartVersion(%s) to return \"%s\", got no error", test.Version, test.ErrorMsg)
		}
	}

	for _, version := range successTest {
		badChart.Version = version
		err := validateChartVersion(badChart)
		if err != nil {
			t.Errorf("validateChartVersion(%s) to return no error, got a linter error", version)
		}
	}
}

func TestValidateChartMaintainer(t *testing.T) {
	var failTest = []struct {
		Name     string
		Email    string
		ErrorMsg string
	}{
		{"", "", "each maintainer requires a name"},
		{"", "test@test.com", "each maintainer requires a name"},
		{"John Snow", "wrongFormatEmail.com", "invalid email"},
	}

	var successTest = []struct {
		Name  string
		Email string
	}{
		{"John Snow", ""},
		{"John Snow", "john@winterfell.com"},
	}

	for _, test := range failTest {
		badChart.Maintainers = []*chart.Maintainer{{Name: test.Name, Email: test.Email}}
		err := validateChartMaintainer(badChart)
		if err == nil || !strings.Contains(err.Error(), test.ErrorMsg) {
			t.Errorf("validateChartMaintainer(%s, %s) to return \"%s\", got no error", test.Name, test.Email, test.ErrorMsg)
		}
	}

	for _, test := range successTest {
		badChart.Maintainers = []*chart.Maintainer{{Name: test.Name, Email: test.Email}}
		err := validateChartMaintainer(badChart)
		if err != nil {
			t.Errorf("validateChartMaintainer(%s, %s) to return no error, got %s", test.Name, test.Email, err.Error())
		}
	}
}

func TestValidateChartSources(t *testing.T) {
	var failTest = []string{"", "RiverRun", "john@winterfell", "riverrun.io"}
	var successTest = []string{"http://riverrun.io", "https://riverrun.io", "https://riverrun.io/blackfish"}
	for _, test := range failTest {
		badChart.Sources = []string{test}
		err := validateChartSources(badChart)
		if err == nil || !strings.Contains(err.Error(), "invalid source URL") {
			t.Errorf("validateChartSources(%s) to return \"invalid source URL\", got no error", test)
		}
	}

	for _, test := range successTest {
		badChart.Sources = []string{test}
		err := validateChartSources(badChart)
		if err != nil {
			t.Errorf("validateChartSources(%s) to return no error, got %s", test, err.Error())
		}
	}
}

func TestValidateChartIconPresence(t *testing.T) {
	err := validateChartIconPresence(badChart)
	if err == nil {
		t.Errorf("validateChartIconPresence to return a linter error, got no error")
	}
}

func TestValidateChartIconURL(t *testing.T) {
	var failTest = []string{"RiverRun", "john@winterfell", "riverrun.io"}
	var successTest = []string{"http://riverrun.io", "https://riverrun.io", "https://riverrun.io/blackfish.png"}
	for _, test := range failTest {
		badChart.Icon = test
		err := validateChartIconURL(badChart)
		if err == nil || !strings.Contains(err.Error(), "invalid icon URL") {
			t.Errorf("validateChartIconURL(%s) to return \"invalid icon URL\", got no error", test)
		}
	}

	for _, test := range successTest {
		badChart.Icon = test
		err := validateChartSources(badChart)
		if err != nil {
			t.Errorf("validateChartIconURL(%s) to return no error, got %s", test, err.Error())
		}
	}
}

func TestChartfile(t *testing.T) {
	t.Run("Chart.yaml basic validity issues", func(t *testing.T) {
		linter := support.Linter{ChartDir: badChartDir}
		Chartfile(&linter)
		msgs := linter.Messages
		expectedNumberOfErrorMessages := 6

		if len(msgs) != expectedNumberOfErrorMessages {
			t.Errorf("Expected %d errors, got %d", expectedNumberOfErrorMessages, len(msgs))
			return
		}

		if !strings.Contains(msgs[0].Err.Error(), "name is required") {
			t.Errorf("Unexpected message 0: %s", msgs[0].Err)
		}

		if !strings.Contains(msgs[1].Err.Error(), "apiVersion is required. The value must be either \"v1\" or \"v2\"") {
			t.Errorf("Unexpected message 1: %s", msgs[1].Err)
		}

		if !strings.Contains(msgs[2].Err.Error(), "version '0.0.0.0' is not a valid SemVer") {
			t.Errorf("Unexpected message 2: %s", msgs[2].Err)
		}

		if !strings.Contains(msgs[3].Err.Error(), "icon is recommended") {
			t.Errorf("Unexpected message 3: %s", msgs[3].Err)
		}

		if !strings.Contains(msgs[4].Err.Error(), "chart type is not valid in apiVersion") {
			t.Errorf("Unexpected message 4: %s", msgs[4].Err)
		}

		if !strings.Contains(msgs[5].Err.Error(), "dependencies are not valid in the Chart file with apiVersion") {
			t.Errorf("Unexpected message 5: %s", msgs[5].Err)
		}
	})

	t.Run("Chart.yaml validity issues due to type mismatch", func(t *testing.T) {
		linter := support.Linter{ChartDir: anotherBadChartDir}
		Chartfile(&linter)
		msgs := linter.Messages
		expectedNumberOfErrorMessages := 3

		if len(msgs) != expectedNumberOfErrorMessages {
			t.Errorf("Expected %d errors, got %d", expectedNumberOfErrorMessages, len(msgs))
			return
		}

		if !strings.Contains(msgs[0].Err.Error(), "version should be of type string") {
			t.Errorf("Unexpected message 0: %s", msgs[0].Err)
		}

		if !strings.Contains(msgs[1].Err.Error(), "version '7.2445e+06' is not a valid SemVer") {
			t.Errorf("Unexpected message 1: %s", msgs[1].Err)
		}

		if !strings.Contains(msgs[2].Err.Error(), "appVersion should be of type string") {
			t.Errorf("Unexpected message 2: %s", msgs[2].Err)
		}
	})
}
