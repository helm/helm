/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package expander

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"
)

var importFileNames = []string{
	"../test/replicatedservice.py",
}

var validFileName = "../test/ValidContent.yaml"
var outputFileName = "../test/ExpectedOutput.yaml"
var archiveFileName = "../test/TestArchive.tar"
var expanderName = "../../../expansion/expansion.py"

type ExpanderTestCase struct {
	Description      string
	TemplateFileName string
	ImportFileNames  []string
	ExpectedError    string
}

func (etc *ExpanderTestCase) GetTemplate(t *testing.T) *common.Template {
	template, err := util.NewTemplateFromFileNames(etc.TemplateFileName, etc.ImportFileNames)
	if err != nil {
		t.Fatalf("cannot create template for test case '%s': %s", etc.Description, err)
	}

	return template
}

func GetOutputString(t *testing.T, description string) string {
	output, err := ioutil.ReadFile(outputFileName)
	if err != nil {
		t.Fatalf("cannot read output file for test case '%s': %s", description, err)
	}

	return string(output)
}

func expandAndVerifyOutput(t *testing.T, actualOutput, description string) {
	actualResult, err := NewExpansionResult(actualOutput)
	if err != nil {
		t.Fatalf("error in test case '%s': %s\n", description, err)
	}

	expectedOutput := GetOutputString(t, description)
	expectedResult, err := NewExpansionResult(expectedOutput)
	if err != nil {
		t.Fatalf("error in test case '%s': %s\n", description, err)
	}

	if !reflect.DeepEqual(actualResult, expectedResult) {
		message := fmt.Sprintf("want:\n%s\nhave:\n%s\n", expectedOutput, actualOutput)
		t.Fatalf("error in test case '%s':\n%s\n", description, message)
	}
}

func testExpandTemplateFromFile(t *testing.T, fileName, baseName string, importFileNames []string,
	constructor func(string, io.Reader, []string) (*common.Template, error)) {
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("cannot open file %s: %s", fileName, err)
	}

	template, err := constructor(baseName, file, importFileNames)
	if err != nil {
		t.Fatalf("cannot create template from file %s: %s", fileName, err)
	}

	backend := NewExpander(expanderName)
	actualOutput, err := backend.ExpandTemplate(template)
	if err != nil {
		t.Fatalf("cannot expand template from file %s: %s", fileName, err)
	}

	description := fmt.Sprintf("test expand template from file: %s", fileName)
	expandAndVerifyOutput(t, actualOutput, description)
}

func TestExpandTemplateFromReader(t *testing.T) {
	baseName := path.Base(validFileName)
	testExpandTemplateFromFile(t, validFileName, baseName, importFileNames, util.NewTemplateFromReader)
}

/*
func TestExpandTemplateFromArchive(t *testing.T) {
	baseName := path.Base(validFileName)
	testExpandTemplateFromFile(t, archiveFileName, baseName, nil, util.NewTemplateFromArchive)
}
*/

var ExpanderTestCases = []ExpanderTestCase{
	{
		"expect error for invalid file name",
		"../test/InvalidFileName.yaml",
		importFileNames,
		"ExpansionError: Exception",
	},
	{
		"expect error for invalid property",
		"../test/InvalidProperty.yaml",
		importFileNames,
		"ExpansionError: Exception",
	},
	{
		"expect error for malformed content",
		"../test/MalformedContent.yaml",
		importFileNames,
		"ExpansionError: Error parsing YAML: mapping values are not allowed here",
	},
	{
		"expect error for missing imports",
		"../test/MissingImports.yaml",
		importFileNames,
		"ExpansionError: Exception",
	},
	{
		"expect error for missing resource name",
		"../test/MissingResourceName.yaml",
		importFileNames,
		"ExpansionError: Resource does not have a name",
	},
	{
		"expect error for missing type name",
		"../test/MissingTypeName.yaml",
		importFileNames,
		"ExpansionError: Resource does not have type defined",
	},
	{
		"expect success",
		validFileName,
		importFileNames,
		"",
	},
}

func TestExpandTemplate(t *testing.T) {
	backend := NewExpander(expanderName)
	for _, etc := range ExpanderTestCases {
		template := etc.GetTemplate(t)
		actualOutput, err := backend.ExpandTemplate(template)
		if err != nil {
			message := err.Error()
			if !strings.Contains(message, etc.ExpectedError) {
				t.Fatalf("error in test case '%s': %s\n", etc.Description, message)
			}
		} else {
			if etc.ExpectedError != "" {
				t.Fatalf("expected error did not occur in test case '%s': %s\n",
					etc.Description, etc.ExpectedError)
			}

			expandAndVerifyOutput(t, actualOutput, etc.Description)
		}
	}
}
