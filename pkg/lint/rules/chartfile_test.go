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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint/support"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const (
	badChartDir  = "testdata/badchartfile"
	goodChartDir = "testdata/goodone"
)

var (
	badChartFilePath         = filepath.Join(badChartDir, "Chart.yaml")
	goodChartFilePath        = filepath.Join(goodChartDir, "Chart.yaml")
	nonExistingChartFilePath = filepath.Join(os.TempDir(), "Chart.yaml")
)

var badChart, chatLoadRrr = chartutil.LoadChartfile(badChartFilePath)
var goodChart, _ = chartutil.LoadChartfile(goodChartFilePath)

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

func TestValidateChartNameDirMatch(t *testing.T) {
	err := validateChartNameDirMatch(goodChartDir, goodChart)
	if err != nil {
		t.Errorf("validateChartNameDirMatch to return no error, gor a linter error")
	}
	// It has not name
	err = validateChartNameDirMatch(badChartDir, badChart)
	if err == nil {
		t.Errorf("validatechartnamedirmatch to return a linter error, got no error")
	}

	// Wrong path
	err = validateChartNameDirMatch(badChartDir, goodChart)
	if err == nil {
		t.Errorf("validatechartnamedirmatch to return a linter error, got no error")
	}
}

func TestValidateChartVersion(t *testing.T) {
	var failTest = []struct {
		Version  string
		ErrorMsg string
	}{
		{"", "version is required"},
		{"0", "0 is less than or equal to 0"},
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

func TestValidateChartEngine(t *testing.T) {
	var successTest = []string{"", "gotpl"}

	for _, engine := range successTest {
		badChart.Engine = engine
		err := validateChartEngine(badChart)
		if err != nil {
			t.Errorf("validateChartEngine(%s) to return no error, got a linter error %s", engine, err.Error())
		}
	}

	badChart.Engine = "foobar"
	err := validateChartEngine(badChart)
	if err == nil || !strings.Contains(err.Error(), "not valid. Valid options are [gotpl") {
		t.Errorf("validateChartEngine(%s) to return an error, got no error", badChart.Engine)
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
	linter := support.Linter{ChartDir: badChartDir}
	Chartfile(&linter)
	msgs := linter.Messages

	if len(msgs) != 4 {
		t.Errorf("Expected 3 errors, got %d", len(msgs))
	}

	if !strings.Contains(msgs[0].Err.Error(), "name is required") {
		t.Errorf("Unexpected message 0: %s", msgs[0].Err)
	}

	if !strings.Contains(msgs[1].Err.Error(), "directory name (badchartfile) and chart name () must be the same") {
		t.Errorf("Unexpected message 1: %s", msgs[1].Err)
	}

	if !strings.Contains(msgs[2].Err.Error(), "version 0.0.0 is less than or equal to 0") {
		t.Errorf("Unexpected message 2: %s", msgs[2].Err)
	}

	if !strings.Contains(msgs[3].Err.Error(), "icon is recommended") {
		t.Errorf("Unexpected message 3: %s", msgs[3].Err)
	}

}
