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
	"path/filepath"

	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path"

	"github.com/ghodss/yaml"
)

// Expander abstracts interactions with the expander and deployer services.
type Expander interface {
	ExpandTemplate(template *Template) (string, error)
}

type expander struct {
	ExpansionBinary string
}

// NewExpander returns a new initialized Expander.
func NewExpander(binary string) Expander {
	return &expander{binary}
}

// ImportFile describes a file that we import into our templates
// TODO: Encode the Content so that it doesn't get mangled.
type ImportFile struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// A Template defines a single deployment.
type Template struct {
	Name    string        `json:"name"`
	Content string        `json:"content"`
	Imports []*ImportFile `json:"imports"`
}

// NewTemplateFromRootTemplate creates and returns a new template whose content
// and imported files are constructed from reading the root template, parsing out
// the imports section and reading the imports from there
func NewTemplateFromRootTemplate(templateFileName string) (*Template, error) {
	templateDir := filepath.Dir(templateFileName)
	content, err := ioutil.ReadFile(templateFileName)
	if err != nil {
		return nil, fmt.Errorf("cannot read template file (%s): %s", err, templateFileName)
	}

	var c map[string]interface{}
	err = yaml.Unmarshal([]byte(content), &c)
	if err != nil {
		log.Fatalf("Cannot parse template: %v", err)
	}

	// For each of the imports, grab the import file
	var imports []string
	if c["imports"] != nil {
		for _, importFile := range c["imports"].([]interface{}) {
			var fileName = importFile.(map[string]interface{})["path"].(string)
			imports = append(imports, templateDir+"/"+fileName)
		}
	}
	return NewTemplateFromFileNames(templateFileName, imports[0:])
}

// NewTemplateFromFileNames creates and returns a new template whose content
// and imported files are read from the supplied file names.
func NewTemplateFromFileNames(
	templateFileName string,
	importFileNames []string,
) (*Template, error) {
	name := path.Base(templateFileName)
	content, err := ioutil.ReadFile(templateFileName)
	if err != nil {
		return nil, fmt.Errorf("cannot read template file (%s): %s", err, templateFileName)
	}

	imports := []*ImportFile{}
	for _, importFileName := range importFileNames {
		importFileData, err := ioutil.ReadFile(importFileName)
		if err != nil {
			return nil, fmt.Errorf("cannot read import file (%s): %s", err, importFileName)
		}

		imports = append(imports,
			&ImportFile{
				Name:    path.Base(importFileName),
				Content: string(importFileData),
			})
	}

	return &Template{
		Name:    name,
		Content: string(content),
		Imports: imports,
	}, nil
}

// ExpansionResult describes the unmarshalled output of ExpandTemplate.
type ExpansionResult struct {
	Config map[string]interface{}
	Layout map[string]interface{}
}

// NewExpansionResult creates and returns a new expansion result from
// the raw output of ExpandTemplate.
func NewExpansionResult(output string) (*ExpansionResult, error) {
	eResponse := &ExpansionResult{}
	if err := yaml.Unmarshal([]byte(output), eResponse); err != nil {
		return nil, fmt.Errorf("cannot unmarshal expansion result (%s):\n%s", err, output)
	}

	return eResponse, nil
}

// Marshal creates and returns an ExpansionResponse from an ExpansionResult.
func (eResult *ExpansionResult) Marshal() (*ExpansionResponse, error) {
	configYaml, err := yaml.Marshal(eResult.Config)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal manifest template (%s):\n%s", err, eResult.Config)
	}

	layoutYaml, err := yaml.Marshal(eResult.Layout)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal manifest layout (%s):\n%s", err, eResult.Layout)
	}

	return &ExpansionResponse{
		Config: string(configYaml),
		Layout: string(layoutYaml),
	}, nil
}

// ExpansionResponse describes the results of marshaling an ExpansionResult.
type ExpansionResponse struct {
	Config string `json:"config"`
	Layout string `json:"layout"`
}

// NewExpansionResponse creates and returns a new expansion response from
// the raw output of ExpandTemplate.
func NewExpansionResponse(output string) (*ExpansionResponse, error) {
	eResult, err := NewExpansionResult(output)
	if err != nil {
		return nil, err
	}

	eResponse, err := eResult.Marshal()
	if err != nil {
		return nil, err
	}

	return eResponse, nil
}

// Unmarshal creates and returns an ExpansionResult from an ExpansionResponse.
func (eResponse *ExpansionResponse) Unmarshal() (*ExpansionResult, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(eResponse.Config), &config); err != nil {
		return nil, fmt.Errorf("cannot unmarshal config (%s):\n%s", err, eResponse.Config)
	}

	var layout map[string]interface{}
	if err := yaml.Unmarshal([]byte(eResponse.Layout), &layout); err != nil {
		return nil, fmt.Errorf("cannot unmarshal layout (%s):\n%s", err, eResponse.Layout)
	}

	return &ExpansionResult{
		Config: config,
		Layout: layout,
	}, nil
}

// ExpandTemplate passes the given configuration to the expander and returns the
// expanded configuration as a string on success.
func (e *expander) ExpandTemplate(template *Template) (string, error) {
	if e.ExpansionBinary == "" {
		message := fmt.Sprintf("expansion binary cannot be empty")
		return "", fmt.Errorf("error expanding template %s: %s", template.Name, message)
	}

	// Those are automatically increasing buffers, so writing arbitrary large
	// data here won't block the child process.
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := &exec.Cmd{
		Path: e.ExpansionBinary,
		// Note, that binary name still has to be passed argv[0].
		Args: []string{e.ExpansionBinary, template.Content},
		// TODO(vagababov): figure out whether do we even need "PROJECT" and
		// "DEPLOYMENT_NAME" variables here.
		Env: []string{
			"PROJECT=" + template.Name,
			"DEPLOYMENT_NAME=" + template.Name,
		},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	for _, imp := range template.Imports {
		cmd.Args = append(cmd.Args, imp.Name, imp.Content)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("error starting expansion process: %s", err)
		return "", err
	}

	cmd.Wait()

	log.Printf("Expansion process: pid: %d SysTime: %v UserTime: %v", cmd.ProcessState.Pid(),
		cmd.ProcessState.SystemTime(), cmd.ProcessState.UserTime())
	if stderr.String() != "" {
		return "", fmt.Errorf("error expanding template %s: %s", template.Name, stderr.String())
	}

	return stdout.String(), nil
}
