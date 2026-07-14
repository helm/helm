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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
)

const (
	badChartNameDir    = "testdata/badchartname"
	badChartDir        = "testdata/badchartfile"
	anotherBadChartDir = "testdata/anotherbadchartfile"
)

var (
	badChartNamePath         = filepath.Join(badChartNameDir, "Chart.yaml")
	badChartFilePath         = filepath.Join(badChartDir, "Chart.yaml")
	nonExistingChartFilePath = filepath.Join(os.TempDir(), "Chart.yaml")
)

var badChart, _ = chartutil.LoadChartfile(badChartFilePath)
var badChartName, _ = chartutil.LoadChartfile(badChartNamePath)

// Validation functions Test
func TestValidateChartYamlNotDirectory(t *testing.T) {
	_ = os.Mkdir(nonExistingChartFilePath, os.ModePerm)
	defer os.Remove(nonExistingChartFilePath)

	err := validateChartYamlNotDirectory(nonExistingChartFilePath)
	assert.Error(t, err, "validateChartYamlNotDirectory to return a linter error, got no error")
}

func TestValidateChartYamlFormat(t *testing.T) {
	err := validateChartYamlFormat(errors.New("Read error"))
	require.Error(t, err, "validateChartYamlFormat to return a linter error, got no error")

	err = validateChartYamlFormat(nil)
	assert.NoError(t, err, "validateChartYamlFormat to return no error, got a linter error")
}

func TestValidateChartName(t *testing.T) {
	err := validateChartName(badChart)
	require.Error(t, err, "validateChartName to return a linter error, got no error")

	err = validateChartName(badChartName)
	assert.Error(t, err, "expected validateChartName to return a linter error for an invalid name, got no error")
}

func TestValidateChartVersion(t *testing.T) {
	var failTest = []struct {
		Version  string
		ErrorMsg string
	}{
		{"", "version is required"},
		{"1.2.3.4", "version '1.2.3.4' is not a valid SemVerV2"},
		{"waps", "'waps' is not a valid SemVerV2"},
		{"-3", "'-3' is not a valid SemVerV2"},
		{"1.1", "'1.1' is not a valid SemVerV2"},
		{"1", "'1' is not a valid SemVerV2"},
	}

	var successTest = []string{"0.0.1", "0.0.1+build", "0.0.1-beta"}

	for i, test := range failTest {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			badChart.Version = test.Version
			err := validateChartVersion(badChart)
			require.ErrorContains(t, err, test.ErrorMsg, "validateChartVersion(%s) to return \"%s\", got no error", test.Version, test.ErrorMsg)
		})
	}

	for _, version := range successTest {
		badChart.Version = version
		err := validateChartVersion(badChart)
		assert.NoError(t, err, "validateChartVersion(%s) to return no error, got a linter error", version)
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
		t.Run(fmt.Sprintf("%s, %s", test.Name, test.Email), func(t *testing.T) {
			badChart.Maintainers = []*chart.Maintainer{{Name: test.Name, Email: test.Email}}
			err := validateChartMaintainer(badChart)
			require.ErrorContains(t, err, test.ErrorMsg, "validateChartMaintainer(%s, %s) to return \"%s\", got no error", test.Name, test.Email, test.ErrorMsg)
		})
	}

	for _, test := range successTest {
		t.Run(fmt.Sprintf("%s, %s", test.Name, test.Email), func(t *testing.T) {
			badChart.Maintainers = []*chart.Maintainer{{Name: test.Name, Email: test.Email}}
			err := validateChartMaintainer(badChart)
			require.NoError(t, err, "validateChartMaintainer(%s, %s) to return no error", test.Name, test.Email)
		})
	}

	// Testing for an empty maintainer
	badChart.Maintainers = []*chart.Maintainer{nil}
	err := validateChartMaintainer(badChart)
	require.Error(t, err, "validateChartMaintainer did not return error for nil maintainer as expected")
	assert.EqualError(t, err, "a maintainer entry is empty", "validateChartMaintainer returned unexpected error for nil maintainer")
}

func TestValidateChartSources(t *testing.T) {
	var failTest = []string{"", "RiverRun", "john@winterfell", "riverrun.io"}
	var successTest = []string{"http://riverrun.io", "https://riverrun.io", "https://riverrun.io/blackfish"}
	for _, test := range failTest {
		t.Run(test, func(t *testing.T) {
			badChart.Sources = []string{test}
			err := validateChartSources(badChart)
			require.ErrorContains(t, err, "invalid source URL", "validateChartSources(%s) to return \"invalid source URL\", got no error", test)
		})
	}

	for _, test := range successTest {
		badChart.Sources = []string{test}
		err := validateChartSources(badChart)
		assert.NoError(t, err, "validateChartSources(%s) to return no error", test)
	}
}

func TestValidateChartIconPresence(t *testing.T) {
	t.Run("Icon absent", func(t *testing.T) {
		testChart := &chart.Metadata{
			Icon: "",
		}

		err := validateChartIconPresence(testChart)

		require.Error(t, err, "validateChartIconPresence to return a linter error, got no error")
		assert.ErrorContains(t, err, "icon is recommended", "expected %q", "icon is recommended")
	})
	t.Run("Icon present", func(t *testing.T) {
		testChart := &chart.Metadata{
			Icon: "http://example.org/icon.png",
		}

		err := validateChartIconPresence(testChart)

		assert.NoError(t, err, "Unexpected error")
	})
}

func TestValidateChartIconURL(t *testing.T) {
	var failTest = []string{"RiverRun", "john@winterfell", "riverrun.io"}
	var successTest = []string{"http://riverrun.io", "https://riverrun.io", "https://riverrun.io/blackfish.png"}
	for _, test := range failTest {
		t.Run(test, func(t *testing.T) {
			badChart.Icon = test
			err := validateChartIconURL(badChart)
			require.ErrorContains(t, err, "invalid icon URL", "validateChartIconURL(%s) to return \"invalid icon URL\", got no error", test)
		})
	}

	for _, test := range successTest {
		badChart.Icon = test
		err := validateChartIconURL(badChart)
		assert.NoError(t, err, "validateChartIconURL(%s) to return no error", test)
	}
}

func TestV3Chartfile(t *testing.T) {
	t.Run("Chart.yaml basic validity issues", func(t *testing.T) {
		linter := support.Linter{ChartDir: badChartDir}
		Chartfile(&linter)
		msgs := linter.Messages
		expectedNumberOfErrorMessages := 6

		require.Lenf(t, msgs, expectedNumberOfErrorMessages, "Expected %d errors", expectedNumberOfErrorMessages)
		require.ErrorContains(t, msgs[0].Err, "name is required", "Unexpected message 0")
		require.ErrorContains(t, msgs[1].Err, "apiVersion is required. The value must be \"v3\"", "Unexpected message 1")
		require.ErrorContains(t, msgs[2].Err, "version '0.0.0.0' is not a valid SemVer", "Unexpected message 2")
		assert.ErrorContains(t, msgs[3].Err, "icon is recommended", "Unexpected message 3")
	})

	t.Run("Chart.yaml validity issues due to type mismatch", func(t *testing.T) {
		linter := support.Linter{ChartDir: anotherBadChartDir}
		Chartfile(&linter)
		msgs := linter.Messages
		expectedNumberOfErrorMessages := 3

		require.Lenf(t, msgs, expectedNumberOfErrorMessages, "Expected %d errors", expectedNumberOfErrorMessages)
		require.ErrorContains(t, msgs[0].Err, "version should be of type string", "Unexpected message 0")
		require.ErrorContains(t, msgs[1].Err, "version '7.2445e+06' is not a valid SemVer", "Unexpected message 1")
		assert.ErrorContains(t, msgs[2].Err, "appVersion should be of type string", "Unexpected message 2")
	})
}
