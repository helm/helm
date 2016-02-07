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

// TODO(jackgr): Mock github repository service to test package and template registry implementations.

import (
	"bytes"
	"io/ioutil"

	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/util"

	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// TestURLAndError associates a URL with an error string for testing.
type TestURLAndError struct {
	URL string
	Err error
}

// DownloadResponse holds a mock http response for testing.
type DownloadResponse struct {
	Err  error
	Code int
	Body string
}

type testGithubRegistryProvider struct {
	shortURL          string
	responses         map[Type]TestURLAndError
	downloadResponses map[string]DownloadResponse
}

type testGithubRegistry struct {
	githubRegistry
	responses         map[Type]TestURLAndError
	downloadResponses map[string]DownloadResponse
}

// NewTestGithubRegistryProvider creates a test github registry provider.
func NewTestGithubRegistryProvider(shortURL string, responses map[Type]TestURLAndError) GithubRegistryProvider {
	return testGithubRegistryProvider{
		shortURL:  util.TrimURLScheme(shortURL),
		responses: responses,
	}
}

// NewTestGithubRegistryProviderWithDownloads creates a test github registry provider with download responses.
func NewTestGithubRegistryProviderWithDownloads(shortURL string, responses map[Type]TestURLAndError, downloadResponses map[string]DownloadResponse) GithubRegistryProvider {
	return testGithubRegistryProvider{
		shortURL:          util.TrimURLScheme(shortURL),
		responses:         responses,
		downloadResponses: downloadResponses,
	}
}

// GetGithubRegistry is a mock implementation of the same method on GithubRegistryProvider.
func (tgrp testGithubRegistryProvider) GetGithubRegistry(cr common.Registry) (GithubRegistry, error) {
	trimmed := util.TrimURLScheme(cr.URL)
	if strings.HasPrefix(trimmed, tgrp.shortURL) {
		ghr, err := newGithubRegistry(cr.Name, trimmed, cr.Format, http.DefaultClient, nil)
		if err != nil {
			panic(fmt.Errorf("cannot create a github registry: %s", err))
		}

		return &testGithubRegistry{
			githubRegistry:    *ghr,
			responses:         tgrp.responses,
			downloadResponses: tgrp.downloadResponses,
		}, nil
	}

	panic(fmt.Errorf("unknown registry: %v", cr))
}

// ListTypes is a mock implementation of the same method on GithubRegistryProvider.
func (tgr testGithubRegistry) ListTypes(regex *regexp.Regexp) ([]Type, error) {
	panic(fmt.Errorf("ListTypes should not be called in the test"))
}

// GetDownloadURLs is a mock implementation of the same method on GithubRegistryProvider.
func (tgr testGithubRegistry) GetDownloadURLs(t Type) ([]*url.URL, error) {
	result := tgr.responses[t]
	URL, err := url.Parse(result.URL)
	if err != nil {
		panic(err)
	}

	return []*url.URL{URL}, result.Err
}

// Do is a mock implementation of the same method on GithubRegistryProvider.
func (tgr testGithubRegistry) Do(req *http.Request) (resp *http.Response, err error) {
	response := tgr.downloadResponses[req.URL.String()]
	return &http.Response{StatusCode: response.Code, Body: ioutil.NopCloser(bytes.NewBufferString(response.Body))}, response.Err
}

type testGCSRegistryProvider struct {
	shortURL  string
	responses map[Type]TestURLAndError
}

type testGCSRegistry struct {
	GCSRegistry
	responses map[Type]TestURLAndError
}

// NewTestGCSRegistryProvider creates a test GCS registry provider.
func NewTestGCSRegistryProvider(shortURL string, responses map[Type]TestURLAndError) GCSRegistryProvider {
	return testGCSRegistryProvider{
		shortURL:  util.TrimURLScheme(shortURL),
		responses: responses,
	}
}

// GetDownloadURLs is a mock implementation of the same method on GCSRegistryProvider.
func (tgrp testGCSRegistryProvider) GetGCSRegistry(cr common.Registry) (ObjectStorageRegistry, error) {
	trimmed := util.TrimURLScheme(cr.URL)
	if strings.HasPrefix(trimmed, tgrp.shortURL) {
		gcsr, err := NewGCSRegistry(cr.Name, trimmed, http.DefaultClient, nil)
		if err != nil {
			panic(fmt.Errorf("cannot create gcs registry: %s", err))
		}

		return &testGCSRegistry{
			GCSRegistry: *gcsr,
			responses:   tgrp.responses,
		}, nil
	}

	panic(fmt.Errorf("unknown registry: %v", cr))
}
