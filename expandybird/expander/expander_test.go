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
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

const invalidFileName = "afilethatdoesnotexist"

var importFileNames = []string{
	"../test/replicatedservice.py",
}

var outputFileName = "../test/ExpectedOutput.yaml"

type ExpanderTestCase struct {
	Description      string
	TemplateFileName string
	ImportFileNames  []string
	ExpectedError    string
}

func (etc *ExpanderTestCase) GetTemplate(t *testing.T) *Template {
	template, err := NewTemplateFromFileNames(etc.TemplateFileName, etc.ImportFileNames)
	if err != nil {
		t.Errorf("cannot create template for test case '%s': %s\n", etc.Description, err)
	}

	return template
}

func GetOutputString(t *testing.T, description string) string {
	output, err := ioutil.ReadFile(outputFileName)
	if err != nil {
		t.Errorf("cannot read output file for test case '%s': %s\n", description, err)
	}

	return string(output)
}

func TestNewTemplateFromFileNames(t *testing.T) {
	if _, err := NewTemplateFromFileNames(invalidFileName, importFileNames); err == nil {
		t.Errorf("expected error did not occur for invalid template file name")
	}

	_, err := NewTemplateFromFileNames(invalidFileName, []string{"afilethatdoesnotexist"})
	if err == nil {
		t.Errorf("expected error did not occur for invalid import file names")
	}
}

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
		"../test/ValidContent.yaml",
		importFileNames,
		"",
	},
}

func TestExpandTemplate(t *testing.T) {
	backend := NewExpander("../expansion/expansion.py")
	for _, etc := range ExpanderTestCases {
		template := etc.GetTemplate(t)
		actualOutput, err := backend.ExpandTemplate(template)
		if err != nil {
			message := err.Error()
			if !strings.Contains(message, etc.ExpectedError) {
				t.Errorf("error in test case '%s': %s\n", etc.Description, message)
			}
		} else {
			if etc.ExpectedError != "" {
				t.Errorf("expected error did not occur in test case '%s': %s\n",
					etc.Description, etc.ExpectedError)
			}

			actualResult, err := NewExpansionResult(actualOutput)
			if err != nil {
				t.Errorf("error in test case '%s': %s\n", etc.Description, err)
			}

			expectedOutput := GetOutputString(t, etc.Description)
			expectedResult, err := NewExpansionResult(expectedOutput)
			if err != nil {
				t.Errorf("error in test case '%s': %s\n", etc.Description, err)
			}

			if !reflect.DeepEqual(actualResult, expectedResult) {
				message := fmt.Sprintf("want: %s\nhave: %s\n", expectedOutput, actualOutput)
				t.Errorf("error in test case '%s': %s\n", etc.Description, message)
			}
		}
	}
}
