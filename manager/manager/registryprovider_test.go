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
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/registry"
)

type urlAndError struct {
	u string
	e error
}

type testRegistryProvider struct {
	r map[string]registry.Registry
}

func newTestRegistryProvider(shortURL string, tests map[registry.Type]urlAndError) registry.RegistryProvider {
	r := make(map[string]registry.Registry)
	r[shortURL] = &testGithubRegistry{tests}
	return testRegistryProvider{r}
}

func (trp testRegistryProvider) GetRegistryByShortURL(URL string) (registry.Registry, error) {
	for key, r := range trp.r {
		if strings.HasPrefix(URL, key) {
			return r, nil
		}
	}

	return nil, fmt.Errorf("No registry found for %s", URL)
}

func (trp testRegistryProvider) GetRegistryByName(registryName string) (registry.Registry, error) {
	panic(fmt.Errorf("GetRegistryByName should not be called in the test"))
}

func (trp testRegistryProvider) GetGithubRegistry(cr common.Registry) (registry.GithubRegistry, error) {
	panic(fmt.Errorf("GetGithubRegistry should not be called in the test"))
}

type testGithubRegistry struct {
	responses map[registry.Type]urlAndError
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

func (tgr testGithubRegistry) GetRegistryOwner() string {
	panic(fmt.Errorf("GetRegistryOwner should not be called in the test"))
}

func (tgr testGithubRegistry) GetRegistryRepository() string {
	panic(fmt.Errorf("GetRegistryRepository should not be called in the test"))
}

func (tgr testGithubRegistry) GetRegistryPath() string {
	panic(fmt.Errorf("GetRegistryPath should not be called in the test"))
}

func (tgr testGithubRegistry) ListTypes(regex *regexp.Regexp) ([]registry.Type, error) {
	panic(fmt.Errorf("ListTypes should not be called in the test"))
}

func (tgr testGithubRegistry) GetDownloadURLs(t registry.Type) ([]*url.URL, error) {
	ret := tgr.responses[t]
	URL, err := url.Parse(ret.u)
	if err != nil {
		panic(err)
	}

	return []*url.URL{URL}, ret.e
}
