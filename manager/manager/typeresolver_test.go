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

package manager

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/registry"
)

type responseAndError struct {
	err  error
	code int
	resp string
}

type resolverTestCase struct {
	config           string
	imports          []*common.ImportFile
	responses        map[string]responseAndError
	urlcount         int
	expectedErr      error
	importOut        []*common.ImportFile
	registryProvider registry.RegistryProvider
}

type testGetter struct {
	responses map[string]responseAndError
	count     int
	test      *testing.T
}

var count = 0

func (tg testGetter) Get(url string) (body string, code int, err error) {
	count = count + 1
	ret := tg.responses[url]
	return ret.resp, ret.code, ret.err
}

func testDriver(c resolverTestCase, t *testing.T) {
	g := &testGetter{test: t, responses: c.responses}
	count = 0
	r := &typeResolver{
		maxUrls: 5,
		rp:      c.registryProvider,
		c:       g,
	}

	conf := &common.Configuration{}
	dataErr := yaml.Unmarshal([]byte(c.config), conf)
	if dataErr != nil {
		panic("bad test data")
	}

	result, err := r.ResolveTypes(conf, c.imports)

	if count != c.urlcount {
		t.Errorf("Expected %d url GETs but only %d found %#v", c.urlcount, g.count, g)
	}

	if (err != nil && c.expectedErr == nil) || (err == nil && c.expectedErr != nil) {
		t.Errorf("Expected error %s but found %s", c.expectedErr, err)
	} else if err != nil && !strings.Contains(err.Error(), c.expectedErr.Error()) {
		t.Errorf("Expected error %s but found %s", c.expectedErr, err)
	}

	resultImport := map[common.ImportFile]bool{}
	expectedImport := map[common.ImportFile]bool{}
	for _, i := range result {
		resultImport[*i] = true
	}

	for _, i := range c.importOut {
		expectedImport[*i] = true
	}

	if !reflect.DeepEqual(resultImport, expectedImport) {
		t.Errorf("Expected imports %+v but found %+v", expectedImport, resultImport)
	}
}

var simpleContent = `
resources:
- name: test
  type: ReplicationController
`

func TestNoImports(t *testing.T) {
	test := resolverTestCase{config: simpleContent}
	testDriver(test, t)
}

var includeImport = `
resources:
- name: foo
  type: foo.py
`

func TestIncludedImport(t *testing.T) {
	imports := []*common.ImportFile{&common.ImportFile{Name: "foo.py"}}
	test := resolverTestCase{
		config:  includeImport,
		imports: imports,
	}
	testDriver(test, t)
}

var templateSingleURL = `
resources:
- name: foo
  type: http://my-fake-url
`

func TestSingleUrl(t *testing.T) {
	finalImports := []*common.ImportFile{&common.ImportFile{Name: "http://my-fake-url", Path: "http://my-fake-url", Content: "my-content"}}

	responses := map[string]responseAndError{
		"http://my-fake-url":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url.schema": responseAndError{nil, http.StatusNotFound, ""},
	}

	test := resolverTestCase{
		config:    templateSingleURL,
		importOut: finalImports,
		urlcount:  2,
		responses: responses,
	}
	testDriver(test, t)
}

func TestSingleUrlWith500(t *testing.T) {
	responses := map[string]responseAndError{
		"http://my-fake-url": responseAndError{nil, http.StatusInternalServerError, "my-content"},
	}

	test := resolverTestCase{
		config:      templateSingleURL,
		urlcount:    1,
		responses:   responses,
		expectedErr: errors.New("Received status code 500"),
	}
	testDriver(test, t)
}

var schema1 = `
imports:
- path: my-next-url
  name: schema-import
`

func TestSingleUrlWithSchema(t *testing.T) {
	finalImports := []*common.ImportFile{
		&common.ImportFile{Name: "http://my-fake-url", Path: "http://my-fake-url", Content: "my-content"},
		&common.ImportFile{Name: "schema-import", Content: "schema-import"},
		&common.ImportFile{Name: "http://my-fake-url.schema", Content: schema1},
	}

	responses := map[string]responseAndError{
		"http://my-fake-url":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url.schema": responseAndError{nil, http.StatusOK, schema1},
		"my-next-url":               responseAndError{nil, http.StatusOK, "schema-import"},
		"my-next-url.schema":        responseAndError{nil, http.StatusNotFound, ""},
	}

	test := resolverTestCase{
		config:    templateSingleURL,
		importOut: finalImports,
		urlcount:  4,
		responses: responses,
	}
	testDriver(test, t)
}

var templateExceedsMax = `
resources:
- name: foo
  type: http://my-fake-url
- name: foo1
  type: http://my-fake-url1
- name: foo2
  type: http://my-fake-url2
- name: foo3
  type: http://my-fake-url3
- name: foo4
  type: http://my-fake-url4
- name: foo5
  type: http://my-fake-url5
`

func TestTooManyImports(t *testing.T) {
	responses := map[string]responseAndError{
		"http://my-fake-url":         responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url.schema":  responseAndError{nil, http.StatusNotFound, ""},
		"http://my-fake-url1":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url1.schema": responseAndError{nil, http.StatusNotFound, ""},
		"http://my-fake-url2":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url2.schema": responseAndError{nil, http.StatusNotFound, ""},
		"http://my-fake-url3":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url3.schema": responseAndError{nil, http.StatusNotFound, ""},
		"http://my-fake-url4":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url4.schema": responseAndError{nil, http.StatusNotFound, ""},
		"http://my-fake-url5":        responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url5.schema": responseAndError{nil, http.StatusNotFound, ""},
	}

	test := resolverTestCase{
		config:      templateExceedsMax,
		urlcount:    10,
		responses:   responses,
		expectedErr: errors.New("Number of imports exceeds maximum of 5"),
	}

	testDriver(test, t)
}

var templateSharesImport = `
resources:
- name: foo
  type: http://my-fake-url
- name: foo1
  type: http://my-fake-url1
`

var schema2 = `
imports:
- path: my-next-url
  name: schema-import-1
`

func TestSharedImport(t *testing.T) {
	finalImports := []*common.ImportFile{
		&common.ImportFile{Name: "http://my-fake-url", Path: "http://my-fake-url", Content: "my-content"},
		&common.ImportFile{Name: "http://my-fake-url1", Path: "http://my-fake-url1", Content: "my-content-1"},
		&common.ImportFile{Name: "schema-import", Content: "schema-import"},
		&common.ImportFile{Name: "schema-import-1", Content: "schema-import"},
		&common.ImportFile{Name: "http://my-fake-url.schema", Content: schema1},
		&common.ImportFile{Name: "http://my-fake-url1.schema", Content: schema2},
	}

	responses := map[string]responseAndError{
		"http://my-fake-url":         responseAndError{nil, http.StatusOK, "my-content"},
		"http://my-fake-url.schema":  responseAndError{nil, http.StatusOK, schema1},
		"http://my-fake-url1":        responseAndError{nil, http.StatusOK, "my-content-1"},
		"http://my-fake-url1.schema": responseAndError{nil, http.StatusOK, schema2},
		"my-next-url":                responseAndError{nil, http.StatusOK, "schema-import"},
		"my-next-url.schema":         responseAndError{nil, http.StatusNotFound, ""},
	}

	test := resolverTestCase{
		config:    templateSharesImport,
		urlcount:  6,
		responses: responses,
		importOut: finalImports,
	}

	testDriver(test, t)
}

var templateShortGithubTemplate = `
resources:
- name: foo
  type: github.com/kubernetes/application-dm-templates/common/replicatedservice:v1
- name: foo1
  type: github.com/kubernetes/application-dm-templates/common/replicatedservice:v2
`

func TestShortGithubUrl(t *testing.T) {
	finalImports := []*common.ImportFile{
		&common.ImportFile{
			Name:    "github.com/kubernetes/application-dm-templates/common/replicatedservice:v1",
			Path:    "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py",
			Content: "my-content"},
		&common.ImportFile{
			Name:    "github.com/kubernetes/application-dm-templates/common/replicatedservice:v2",
			Path:    "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py",
			Content: "my-content-2"},
	}

	downloadResponses := map[string]registry.DownloadResponse{
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py":        registry.DownloadResponse{Err: nil, Code: http.StatusOK, Body: "my-content"},
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py.schema": registry.DownloadResponse{Err: nil, Code: http.StatusNotFound, Body: ""},
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py":        registry.DownloadResponse{Err: nil, Code: http.StatusOK, Body: "my-content-2"},
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py.schema": registry.DownloadResponse{Err: nil, Code: http.StatusNotFound, Body: ""},
	}

	githubURLMaps := map[registry.Type]registry.TestURLAndError{
		registry.NewTypeOrDie("common", "replicatedservice", "v1"): registry.TestURLAndError{URL: "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", Err: nil},
		registry.NewTypeOrDie("common", "replicatedservice", "v2"): registry.TestURLAndError{URL: "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py", Err: nil},
	}

	gcsURLMaps := map[registry.Type]registry.TestURLAndError{
		registry.NewTypeOrDie("common", "replicatedservice", "v1"): registry.TestURLAndError{URL: "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", Err: nil},
		registry.NewTypeOrDie("common", "replicatedservice", "v2"): registry.TestURLAndError{URL: "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py", Err: nil},
	}

	grp := registry.NewTestGithubRegistryProviderWithDownloads("github.com/kubernetes/application-dm-templates", githubURLMaps, downloadResponses)
	gcsrp := registry.NewTestGCSRegistryProvider("gs://charts", gcsURLMaps)
	test := resolverTestCase{
		config:           templateShortGithubTemplate,
		importOut:        finalImports,
		urlcount:         0,
		responses:        map[string]responseAndError{},
		registryProvider: registry.NewRegistryProvider(nil, grp, gcsrp, registry.NewInmemCredentialProvider()),
	}

	testDriver(test, t)
}
