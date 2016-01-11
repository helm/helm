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

package registry

// TODO(jackgr): Finish implementing registry provider tests.

import (
	"github.com/kubernetes/deployment-manager/common"

	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

type urlAndError struct {
	u string
	e error
}

type testRegistryProvider struct {
	URLPrefix string
	r         map[string]Registry
}

func newTestRegistryProvider(URLPrefix string, tests map[Type]urlAndError) RegistryProvider {
	r := make(map[string]Registry)
	r[URLPrefix] = testGithubRegistry{tests}
	return testRegistryProvider{URLPrefix, r}
}

func (trp testRegistryProvider) GetRegistryByShortURL(URL string) (Registry, error) {
	for key, r := range trp.r {
		if strings.HasPrefix(URL, key) {
			return r, nil
		}
	}

	return nil, fmt.Errorf("No registry found for %s", URL)
}

func (trp testRegistryProvider) GetRegistryByName(registryName string) (Registry, error) {
	panic(fmt.Errorf("GetRegistryByName should not be called in the test"))
}

func (trp testRegistryProvider) GetGithubRegistry(cr common.Registry) (GithubRegistry, error) {
	panic(fmt.Errorf("GetGithubRegistry should not be called in the test"))
}

type testGithubRegistry struct {
	responses map[Type]urlAndError
}

func (tgr testGithubRegistry) GetRegistryName() string {
	panic(fmt.Errorf("GetRegistryName should not be called in the test"))
}

func (tgr testGithubRegistry) GetRegistryType() common.RegistryType {
	return common.GithubRegistryType
}

func (tgr testGithubRegistry) GetRegistryShortURL() string {
	panic(fmt.Errorf("GetRegistryShortURL should not be called in the test"))
}

func (tgr testGithubRegistry) GetRegistryFormat() common.RegistryFormat {
	panic(fmt.Errorf("GetRegistryFormat should not be called in the test"))
}

func (tgr testGithubRegistry) GetDownloadURLs(t Type) ([]*url.URL, error) {
	ret := tgr.responses[t]
	URL, err := url.Parse(ret.u)
	if err != nil {
		panic(err)
	}

	return []*url.URL{URL}, ret.e
}

func (tgr testGithubRegistry) ListTypes(regex *regexp.Regexp) ([]Type, error) {
	panic(fmt.Errorf("ListTypes should not be called in the test"))
}

func testUrlConversionDriver(rp RegistryProvider, tests map[string]urlAndError, t *testing.T) {
	for in, expected := range tests {
		actual, err := GetDownloadURLs(rp, in)
		if err != expected.e {
			t.Errorf("failed on: %s : expected error %v but got %v", in, expected.e, err)
		}

		if actual[0] != expected.u {
			t.Errorf("failed on: %s : expected %s but got %v", in, expected.u, actual)
		}
	}
}

func TestShortGithubUrlMapping(t *testing.T) {
	githubUrlMaps := map[Type]urlAndError{
		NewTypeOrDie("common", "replicatedservice", "v1"): urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		NewTypeOrDie("storage", "redis", "v1"):            urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/storage/redis/v1/redis.jinja", nil},
	}

	tests := map[string]urlAndError{
		"github.com/kubernetes/application-dm-templates/common/replicatedservice:v1": urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		"github.com/kubernetes/application-dm-templates/storage/redis:v1":            urlAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/storage/redis/v1/redis.jinja", nil},
	}

	test := newTestRegistryProvider("github.com/kubernetes/application-dm-templates", githubUrlMaps)
	testUrlConversionDriver(test, tests, t)
}

func TestShortGithubUrlMappingDifferentOwnerAndRepo(t *testing.T) {
	githubUrlMaps := map[Type]urlAndError{
		NewTypeOrDie("common", "replicatedservice", "v1"): urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		NewTypeOrDie("storage", "redis", "v1"):            urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/storage/redis/v1/redis.jinja", nil},
	}

	tests := map[string]urlAndError{
		"github.com/example/mytemplates/common/replicatedservice:v1": urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		"github.com/example/mytemplates/storage/redis:v1":            urlAndError{"https://raw.githubusercontent.com/example/mytemplates/master/storage/redis/v1/redis.jinja", nil},
	}

	test := newTestRegistryProvider("github.com/example/mytemplates", githubUrlMaps)
	testUrlConversionDriver(test, tests, t)
}
