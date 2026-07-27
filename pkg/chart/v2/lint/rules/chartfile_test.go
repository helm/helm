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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
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
	assert.Error(t, validateChartYamlNotDirectory(nonExistingChartFilePath), "validateChartYamlNotDirectory to return a linter error, got no error")
}

func TestValidateChartYamlFormat(t *testing.T) {
	require.Error(t, validateChartYamlFormat(errors.New("Read error")), "validateChartYamlFormat to return a linter error, got no error")
	assert.NoError(t, validateChartYamlFormat(nil), "validateChartYamlFormat to return no error, got a linter error")
}

func TestValidateChartName(t *testing.T) {
	require.Error(t, validateChartName(badChart), "validateChartName to return a linter error, got no error")
	assert.Error(t, validateChartName(badChartName), "expected validateChartName to return a linter error for an invalid name, got no error")
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
		require.ErrorContainsf(t, validateChartVersion(badChart), test.ErrorMsg, "validateChartVersion(%s) to return \"%s\", got no error", test.Version, test.ErrorMsg)
	}

	for _, version := range successTest {
		badChart.Version = version
		assert.NoError(t, validateChartVersion(badChart), "validateChartVersion(%s) to return no error, got a linter error", version)
	}
}

func TestValidateChartVersionStrictSemVerV2(t *testing.T) {
	var failTest = []struct {
		Version  string
		ErrorMsg string
	}{
		{"", "version '' is not a valid SemVerV2"},
		{"1", "version '1' is not a valid SemVerV2"},
		{"1.1", "version '1.1' is not a valid SemVerV2"},
	}

	var successTest = []string{"1.1.1", "0.0.1+build", "0.0.1-beta"}

	for _, test := range failTest {
		badChart.Version = test.Version
		require.ErrorContainsf(t, validateChartVersionStrictSemVerV2(badChart), test.ErrorMsg, "validateChartVersionStrictSemVerV2(%s) to return \"%s\", got no error", test.Version, test.ErrorMsg)
	}

	for _, version := range successTest {
		badChart.Version = version
		assert.NoError(t, validateChartVersionStrictSemVerV2(badChart), "validateChartVersionStrictSemVerV2(%s) to return no error, got a linter error", version)
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
		require.ErrorContainsf(t, validateChartMaintainer(badChart), test.ErrorMsg, "validateChartMaintainer(%s, %s) to return \"%s\", got no error", test.Name, test.Email, test.ErrorMsg)
	}

	for _, test := range successTest {
		badChart.Maintainers = []*chart.Maintainer{{Name: test.Name, Email: test.Email}}
		require.NoError(t, validateChartMaintainer(badChart), "validateChartMaintainer(%s, %s)", test.Name, test.Email)
	}

	// Testing for an empty maintainer
	badChart.Maintainers = []*chart.Maintainer{nil}
	assert.EqualError(t, validateChartMaintainer(badChart), "a maintainer entry is empty")
}

func TestValidateChartSources(t *testing.T) {
	var failTest = []string{"", "RiverRun", "john@winterfell", "riverrun.io"}
	var successTest = []string{"http://riverrun.io", "https://riverrun.io", "https://riverrun.io/blackfish"}
	for _, test := range failTest {
		badChart.Sources = []string{test}
		require.ErrorContainsf(t, validateChartSources(badChart), "invalid source URL", "validateChartSources(%s) to return \"invalid source URL\", got no error", test)
	}

	for _, test := range successTest {
		badChart.Sources = []string{test}
		assert.NoError(t, validateChartSources(badChart), "validateChartSources(%s) to return no error", test)
	}
}

func TestValidateChartIconPresence(t *testing.T) {
	t.Run("Icon absent", func(t *testing.T) {
		testChart := &chart.Metadata{
			Icon: "",
		}

		assert.ErrorContainsf(t, validateChartIconPresence(testChart), "icon is recommended", "expected %q", "icon is recommended")
	})
	t.Run("Icon present", func(t *testing.T) {
		testChart := &chart.Metadata{
			Icon: "http://example.org/icon.png",
		}
		assert.NoError(t, validateChartIconPresence(testChart))
	})
}

func TestValidateChartIconURL(t *testing.T) {
	var failTest = []string{"RiverRun", "john@winterfell", "riverrun.io"}
	var successTest = []string{"http://riverrun.io", "https://riverrun.io", "https://riverrun.io/blackfish.png"}
	for _, test := range failTest {
		badChart.Icon = test
		require.ErrorContainsf(t, validateChartIconURL(badChart), "invalid icon URL", "validateChartIconURL(%s) to return \"invalid icon URL\", got no error", test)
	}

	for _, test := range successTest {
		badChart.Icon = test
		assert.NoError(t, validateChartSources(badChart), "validateChartIconURL(%s) to return no error", test)
	}
}

func TestChartfile(t *testing.T) {
	t.Run("Chart.yaml basic validity issues", func(t *testing.T) {
		linter := support.Linter{ChartDir: badChartDir}
		Chartfile(&linter)
		msgs := linter.Messages
		expectedNumberOfErrorMessages := 7

		require.Lenf(t, msgs, expectedNumberOfErrorMessages, "Expected %d errors, got %d", expectedNumberOfErrorMessages, len(msgs))
		require.ErrorContains(t, msgs[0].Err, "name is required", "Unexpected message 0: %s", msgs[0].Err)
		require.ErrorContains(t, msgs[1].Err, "apiVersion is required. The value must be either \"v1\" or \"v2\"", "Unexpected message 1: %s", msgs[1].Err)
		require.ErrorContains(t, msgs[2].Err, "version '0.0.0.0' is not a valid SemVer", "Unexpected message 2: %s", msgs[2].Err)
		require.ErrorContains(t, msgs[3].Err, "icon is recommended", "Unexpected message 3: %s", msgs[3].Err)
		require.ErrorContains(t, msgs[4].Err, "chart type is not valid in apiVersion", "Unexpected message 4: %s", msgs[4].Err)
		require.ErrorContains(t, msgs[5].Err, "dependencies are not valid in the Chart file with apiVersion", "Unexpected message 5: %s", msgs[5].Err)
		assert.ErrorContains(t, msgs[6].Err, "version '0.0.0.0' is not a valid SemVerV2", "Unexpected message 6: %s", msgs[6].Err)
	})

	t.Run("Chart.yaml validity issues due to type mismatch", func(t *testing.T) {
		linter := support.Linter{ChartDir: anotherBadChartDir}
		Chartfile(&linter)
		msgs := linter.Messages
		expectedNumberOfErrorMessages := 4

		require.Len(t, msgs, expectedNumberOfErrorMessages, "Expected %d errors, got %d", expectedNumberOfErrorMessages, len(msgs))
		require.ErrorContains(t, msgs[0].Err, "version should be of type string", "Unexpected message 0: %s", msgs[0].Err)
		require.ErrorContains(t, msgs[1].Err, "version '7.2445e+06' is not a valid SemVer", "Unexpected message 1: %s", msgs[1].Err)
		require.ErrorContains(t, msgs[2].Err, "appVersion should be of type string", "Unexpected message 2: %s", msgs[2].Err)
		assert.ErrorContains(t, msgs[3].Err, "version '7.2445e+06' is not a valid SemVerV2", "Unexpected message 3: %s", msgs[3].Err)
	})
}
