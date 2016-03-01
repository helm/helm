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

package util

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/deployment-manager/pkg/common"
)

// NewTemplateFromType creates and returns a new template whose content
// is a YAML marshaled resource assembled from the supplied arguments.
func NewTemplateFromType(name, typeName string, properties map[string]interface{}) (*common.Template, error) {
	resource := &common.Resource{
		Name:       name,
		Type:       typeName,
		Properties: properties,
	}

	config := common.Configuration{Resources: []*common.Resource{resource}}
	content, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("error: %s\ncannot marshal configuration: %v\n", err, config)
	}

	template := &common.Template{
		Name:    name,
		Content: string(content),
		Imports: []*common.ImportFile{},
	}

	return template, nil
}

// NewTemplateFromArchive creates and returns a new template whose content
// and imported files are read from the supplied archive.
func NewTemplateFromArchive(name string, r io.Reader, importFileNames []string) (*common.Template, error) {
	var content []byte
	imports, err := collectImportFiles(importFileNames)
	if err != nil {
		return nil, err
	}

	tr := tar.NewReader(r)
	for i := 0; true; i++ {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if hdr.Name != name {
			importFileData, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("cannot read archive file %s: %s", hdr.Name, err)
			}

			imports = append(imports,
				&common.ImportFile{
					Name:    path.Base(hdr.Name),
					Content: string(importFileData),
				})
		} else {
			content, err = ioutil.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("cannot read %s from archive: %s", name, err)
			}
		}
	}

	if len(content) < 1 {
		return nil, fmt.Errorf("cannot find %s in archive", name)
	}

	return &common.Template{
		Name:    name,
		Content: string(content),
		Imports: imports,
	}, nil
}

// NewTemplateFromReader creates and returns a new template whose content
// is read from the supplied reader.
func NewTemplateFromReader(name string, r io.Reader, importFileNames []string) (*common.Template, error) {
	content, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read archive %s: %s", name, err)
	}

	return newTemplateFromContentAndImports(name, string(content), importFileNames)
}

// NewTemplateFromRootTemplate creates and returns a new template whose content
// and imported files are constructed from reading the root template, parsing out
// the imports section and reading the imports from there
func NewTemplateFromRootTemplate(templateFileName string) (*common.Template, error) {
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
) (*common.Template, error) {
	content, err := ioutil.ReadFile(templateFileName)
	if err != nil {
		return nil, fmt.Errorf("cannot read template file %s: %s", templateFileName, err)
	}

	name := path.Base(templateFileName)
	return newTemplateFromContentAndImports(name, string(content), importFileNames)
}

func newTemplateFromContentAndImports(
	name, content string,
	importFileNames []string,
) (*common.Template, error) {
	if len(content) < 1 {
		return nil, fmt.Errorf("supplied configuration is empty")
	}

	imports, err := collectImportFiles(importFileNames)
	if err != nil {
		return nil, err
	}

	return &common.Template{
		Name:    name,
		Content: content,
		Imports: imports,
	}, nil
}

func collectImportFiles(importFileNames []string) ([]*common.ImportFile, error) {
	imports := []*common.ImportFile{}
	for _, importFileName := range importFileNames {
		importFileData, err := ioutil.ReadFile(importFileName)
		if err != nil {
			return nil, fmt.Errorf("cannot read import file %s: %s", importFileName, err)
		}

		imports = append(imports,
			&common.ImportFile{
				Name:    path.Base(importFileName),
				Content: string(importFileData),
			})
	}

	return imports, nil
}
