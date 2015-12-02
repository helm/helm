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
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/deployment-manager/registry"
	"github.com/kubernetes/deployment-manager/common"
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

func (tg *testGetter) Get(url string) (body string, code int, err error) {
	tg.count = tg.count + 1
	ret := tg.responses[url]

	return ret.resp, ret.code, ret.err
}

type urlAndError struct {
	u string
	e error
}

type testRegistryProvider struct {
	owner string
	repo  string
	r     map[string]registry.Registry
}

func newTestRegistryProvider(owner string, repository string, tests map[registry.Type]urlAndError, count int) registry.RegistryProvider {
	r := make(map[string]registry.Registry)
	r[owner+repository] = &testGithubRegistry{tests, count}
	return &testRegistryProvider{owner, repository, r}
}

func (trp *testRegistryProvider) GetGithubRegistry(owner string, repository string) registry.Registry {
	return trp.r[owner+repository]
}

type testGithubRegistry struct {
	responses map[registry.Type]urlAndError
	count     int
}

func (tgr *testGithubRegistry) GetURL(t registry.Type) (string, error) {
	tgr.count = tgr.count + 1
	ret := tgr.responses[t]
	return ret.u, ret.e
}

func (tgr *testGithubRegistry) List() ([]registry.Type, error) {
	return []registry.Type{}, fmt.Errorf("List should not be called in the test")
}

func testUrlConversionDriver(c resolverTestCase, tests map[string]urlAndError, t *testing.T) {
	r := &typeResolver{
		rp: c.registryProvider,
	}
	for in, expected := range tests {
		actual, err := r.ShortTypeToDownloadURL(in)
		if actual != expected.u {
			t.Errorf("failed on: %s : expected %s but got %s", in, expected.u, actual)
		}
		if err != expected.e {
			t.Errorf("failed on: %s : expected error %v but got %v", in, expected.e, err)
		}
	}
}

func testDriver(c resolverTestCase, t *testing.T) {
	g := &testGetter{test: t, responses: c.responses}
	r := &typeResolver{
		getter:  g,
		maxUrls: 5,
		rp:      c.registryProvider,
	}

	conf := &common.Configuration{}
	dataErr := yaml.Unmarshal([]byte(c.config), conf)
	if dataErr != nil {
		panic("bad test data")
	}

	result, err := r.ResolveTypes(conf, c.imports)

	if g.count != c.urlcount {
		t.Errorf("Expected %d url GETs but only %d found", c.urlcount, g.count)
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

func TestShortGithubUrlMapping(t *testing.T) {
	githubUrlMaps := map[registry.Type]urlAndError{
		registry.Type{"common", "replicatedservice", "v1"}: urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		registry.Type{"storage", "redis", "v1"}:               urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/storage/redis/v1/redis.jinja", nil},
	}

	tests := map[string]urlAndError{
		"github.com/kubernetes/application-dm-templates/common/replicatedservice:v1": urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		"github.com/kubernetes/application-dm-templates/storage/redis:v1":               urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/storage/redis/v1/redis.jinja", nil},
	}

	test := resolverTestCase{
		registryProvider: newTestRegistryProvider("kubernetes", "application-dm-templates", githubUrlMaps, 2),
	}
	testUrlConversionDriver(test, tests, t)
}

func TestShortGithubUrlMappingDifferentOwnerAndRepo(t *testing.T) {
	githubUrlMaps := map[registry.Type]urlAndError{
		registry.Type{"common", "replicatedservice", "v1"}: urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		registry.Type{"storage", "redis", "v1"}:               urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/storage/redis/v1/redis.jinja", nil},
	}

	tests := map[string]urlAndError{
		"github.com/example/mytemplates/common/replicatedservice:v1": urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		"github.com/example/mytemplates/storage/redis:v1":               urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/storage/redis/v1/redis.jinja", nil},
	}

	test := resolverTestCase{
		registryProvider: newTestRegistryProvider("example", "mytemplates", githubUrlMaps, 2),
	}
	testUrlConversionDriver(test, tests, t)
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
			Name: "github.com/kubernetes/application-dm-templates/common/replicatedservice:v1",
			Path: "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py",
			Content: "my-content"},
		&common.ImportFile{
			Name: "github.com/kubernetes/application-dm-templates/common/replicatedservice:v2",
			Path: "https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py",
			Content: "my-content-2"},
	}

	responses := map[string]responseAndError{
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py":        responseAndError{nil, http.StatusOK, "my-content"},
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py.schema": responseAndError{nil, http.StatusNotFound, ""},
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py":        responseAndError{nil, http.StatusOK, "my-content-2"},
		"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py.schema": responseAndError{nil, http.StatusNotFound, ""},
	}

	githubUrlMaps := map[registry.Type]urlAndError{
		registry.Type{"common", "replicatedservice", "v1"}: urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		registry.Type{"common", "replicatedservice", "v2"}: urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v2/replicatedservice.py", nil},
	}

	test := resolverTestCase{
		config:           templateShortGithubTemplate,
		importOut:        finalImports,
		urlcount:         4,
		responses:        responses,
		registryProvider: newTestRegistryProvider("kubernetes", "application-dm-templates", githubUrlMaps, 2),
	}
	testDriver(test, t)
}
